package fileparameter

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/temporalio/maru/target/fileparameter/activities"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

var (
	ErrSessionHostDown = errors.New("session host is down")
)

const (
	shortActivityTimeout  = 1 * time.Minute
	mediumActivityTimeout = 5 * time.Minute
	longActivityTimeout   = 15 * time.Minute

	maxAttempts = 3
)

type Duration struct {
	time.Duration
}

func (duration *Duration) UnmarshalJSON(b []byte) error {
	var unmarshalledJson interface{}

	err := json.Unmarshal(b, &unmarshalledJson)
	if err != nil {
		return err
	}

	switch value := unmarshalledJson.(type) {
	case float64:
		duration.Duration = time.Duration(value)
	case string:
		duration.Duration, err = time.ParseDuration(value)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("invalid duration: %#v", unmarshalledJson)
	}

	return nil
}

// WorkflowRequest is used for starting workflow for Basic bench workflow
type WorkflowRequest struct {
	MinByteFileSize int      `json:"min_byte_file_size"`
	MaxByteFileSize int      `json:"max_byte_file_size"`
	CoresCount      int      `json:"cores_count"`
	CPUTime         Duration `json:"cpu_time"`
	CPUPercentage   int      `json:"cpu_percentage"`
}

// RunWorkflow is the main workflow. It orchestrates various activities
type RunWorkflow struct {
	// SessionCreationTimeout controls how long the overall workflow may take before being canceled.
	ExecutionTimeout time.Duration

	// SessionCreationTimeout controls time allowed to start the Temporal session internal activity.
	SessionCreationTimeout time.Duration
	// SessionRetryInterval controls the time between workflow session creation attempts.
	// See SessionMaxAttempts for details.
	SessionRetryInterval time.Duration
	// SessionMaxAttempts is the maximum number of attempts to create the workflow session following an ErrSessionHostDown error.
	SessionMaxAttempts int
	// SessionHeartbeatTimeout determines how long Temporal will wait between workflow session heartbeats
	// before canceling the session and retrying the workflow with another session.
	SessionHeartbeatTimeout time.Duration
}

// ProcessingWorkflow workflow definition
func (wf *RunWorkflow) ProcessingWorkflow(ctx workflow.Context, request WorkflowRequest) (err error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Minute,
		HeartbeatTimeout:    4 * time.Second, // such a short timeout to make sample fail over very fast
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Retry the whole sequence from the first activity on any error
	// to retry it on a different host. In a real application it might be reasonable to
	// retry individual activities as well as the whole sequence discriminating between different types of errors.
	// See the retryactivity sample for a more sophisticated retry implementation.
	for i := 1; i < wf.SessionMaxAttempts; i++ {
		executionInfo := workflow.GetInfo(ctx).WorkflowExecution
		err = wf.processFile(ctx, executionInfo, request)
		if errors.Is(err, ErrSessionHostDown) {
			if sleepErr := workflow.Sleep(ctx, wf.SessionRetryInterval); sleepErr != nil {
				return sleepErr
			}
			continue
		}
		if err != nil {
			workflow.GetLogger(ctx).Error("Workflow failed.", "Error", err.Error())
		} else {
			workflow.GetLogger(ctx).Info("Workflow completed.")
		}
		return err
	}
	return err
}

func (wf *RunWorkflow) processFile(ctx workflow.Context, executionInfo workflow.Execution, request WorkflowRequest) (err error) {
	workflowLogger := workflow.GetLogger(ctx)

	sessionCtx, err := workflow.CreateSession(ctx, &workflow.SessionOptions{
		CreationTimeout:  wf.SessionCreationTimeout,
		ExecutionTimeout: wf.ExecutionTimeout,
		HeartbeatTimeout: wf.SessionHeartbeatTimeout, // we rely purely on session heartbeats to catch a worker instance going down
	})

	if err != nil {
		err = fmt.Errorf("creating Temporal session: %w", err)
		if temporal.IsTimeoutError(err) {
			// session host may be down: try again up to configured attempts
			err = errors.Join(err, ErrSessionHostDown)
		}
		return err
	}

	var workDir activities.CreateWorkDirResult

	defer func() {
		workflow.CompleteSession(sessionCtx)
		if workflow.GetSessionInfo(sessionCtx).SessionState == workflow.SessionStateFailed {
			// session host is down: signal this to the caller, so it can retry the session workflow
			err = errors.Join(err, ErrSessionHostDown)
		}
	}()

	// prepare context
	workDir, err = wf.createWorkDir(sessionCtx, activities.CreateWorkDirArgs{})
	if err != nil {
		return err
	}
	// Expected Session will leak file on disk, until cleanup activities is implemented
	defer func() {
		cleanupCtx, _ := workflow.NewDisconnectedContext(ctx)
		_, cleanupErr := wf.cleanupWorkDir(cleanupCtx, activities.CleanupWorkDirArgs{
			Root:          workDir.Root,
			HostTaskQueue: workDir.HostTaskQueue,
		})
		if cleanupErr != nil {
			workflowLogger.Warn("Workflow Cleanup failed...", "Error", cleanupErr)
		}
	}()

	_, err = wf.generateData(sessionCtx, activities.GenerateDataParams{
		ContextDir:  workDir.ContextDir,
		MinFileSize: request.MinByteFileSize,
		MaxFileSize: request.MaxByteFileSize,
	})
	if err != nil {
		return err
	}
	_, err = wf.processData(sessionCtx, activities.ProcessDataParams{
		ContextDir: workDir.ContextDir,
		ResultDir:  workDir.ResultDir,
		CoresCount: request.CoresCount,
		Duration:   request.CPUTime.Duration,
		Percentage: request.CPUPercentage,
	})
	if err != nil {
		return err
	}
	result, err := wf.uploadResult(sessionCtx, activities.UploadParams{
		ResultDir:  workDir.ResultDir,
		WorkflowID: executionInfo.ID,
	})
	if err != nil {
		return err
	}

	workflowLogger.Info("Workflow Completed...", "file", result.ResultLocation)
	return err
}

func (wf *RunWorkflow) createWorkDir(ctx workflow.Context, args activities.CreateWorkDirArgs) (activities.CreateWorkDirResult, error) {
	activityCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: shortActivityTimeout,
		RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: maxAttempts},
	})
	var a *activities.WorkDir
	f := workflow.ExecuteActivity(activityCtx, a.CreateWorkDir, args)
	var result activities.CreateWorkDirResult
	return result, f.Get(ctx, &result)
}

func (wf *RunWorkflow) cleanupWorkDir(cleanupCtx workflow.Context, args activities.CleanupWorkDirArgs) (result activities.CleanupWorkDirResult, err error) {
	// Cleanup Timeout what do we do.
	var a *activities.WorkDir

	activityCtx := workflow.WithActivityOptions(cleanupCtx, workflow.ActivityOptions{
		StartToCloseTimeout:    shortActivityTimeout,
		ScheduleToCloseTimeout: 20 * time.Second,
		TaskQueue:              args.HostTaskQueue,
	})

	f := workflow.ExecuteActivity(activityCtx, a.CleanupWorkDir, args)
	return result, f.Get(cleanupCtx, &result)
}

func (wf *RunWorkflow) generateData(ctx workflow.Context, args activities.GenerateDataParams) (result activities.GenerateDataResult, err error) {
	activityCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: shortActivityTimeout,
		RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: maxAttempts},
	})

	f := workflow.ExecuteActivity(activityCtx, activities.GenerateData, args)

	return result, f.Get(ctx, &result)
}

func (wf *RunWorkflow) processData(ctx workflow.Context, args activities.ProcessDataParams) (result activities.ProcessDataResult, err error) {
	activityCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: longActivityTimeout,
		RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: maxAttempts},
	})

	f := workflow.ExecuteActivity(activityCtx, activities.ProcessData, args)

	return result, f.Get(ctx, &result)
}

func (wf *RunWorkflow) uploadResult(ctx workflow.Context, args activities.UploadParams) (result activities.UploadResult, err error) {
	activityCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: mediumActivityTimeout,
		RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: maxAttempts},
	})
	var upload *activities.Upload

	f := workflow.ExecuteActivity(activityCtx, upload.UploadActivity, args)

	return result, f.Get(ctx, &result)
}
