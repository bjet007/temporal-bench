package activities

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"go.temporal.io/sdk/activity"
)

type CreateWorkDirArgs struct {
}

type CreateWorkDirResult struct {
	// Root is the working directory for the whole process. Contains all other dirs in this result.
	Root string
	// ContextDir will hold context files.
	ContextDir string
	// ResultDir will hold any result files, including estimation details and logs.
	ResultDir string
}

// CreateWorkDir creates the working directory as well as its children (context & results).
type CreateWorkDir struct {
	BaseDir string
}

func (a *CreateWorkDir) CreateWorkDir(ctx context.Context, _ CreateWorkDirArgs) (*CreateWorkDirResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Creating working directory")

	workDir, err := os.MkdirTemp(a.BaseDir, "workflow-*")
	if err != nil {
		return nil, err
	}

	contextDir := filepath.Join(workDir, "process-context")
	if err := os.MkdirAll(contextDir, 0755|os.ModeDir); err != nil {
		return nil, fmt.Errorf("creating context directory (%q): %w", contextDir, err)
	}

	resultDir := filepath.Join(workDir, "process-result")
	if err := os.MkdirAll(resultDir, 0755|os.ModeDir); err != nil {
		return nil, fmt.Errorf("creating result directory (%q): %w", resultDir, err)
	}

	return &CreateWorkDirResult{
		Root:       workDir,
		ContextDir: contextDir,
		ResultDir:  resultDir,
	}, nil
}
