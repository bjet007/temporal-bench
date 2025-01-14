package activities

import (
	"context"
	"path/filepath"

	"go.temporal.io/sdk/activity"
)

type UploadParams struct {
	ResultDir  string
	WorkflowID string
}

type UploadResult struct {
	ResultLocation string
}

type Upload struct {
	BlobStore *BlobStore
}

func (a *Upload) UploadActivity(ctx context.Context, args UploadParams) (*UploadResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Uploading result...", "ResultDir", args.ResultDir)

	join := filepath.Join(args.ResultDir, ChecksumFile)
	objectFileName, err := a.BlobStore.uploadFile(ctx, args.WorkflowID, join)
	if err != nil {
		logger.Error("uploadFileActivity uploading failed.", "Error", err)
		return nil, err
	}
	logger.Info("Uploading succeed.", "UploadedFileName", *objectFileName)
	return &UploadResult{
		ResultLocation: *objectFileName,
	}, nil
}
