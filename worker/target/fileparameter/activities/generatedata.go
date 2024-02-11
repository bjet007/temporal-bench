package activities

import (
	"context"
	"crypto/rand"
	math_rand "math/rand"
	"path/filepath"

	"go.temporal.io/sdk/activity"
)

type GenerateDataParams struct {
	ContextDir  string
	MinFileSize int
	MaxFileSize int
}

type GenerateDataResult struct {
}

func GenerateData(ctx context.Context, args GenerateDataParams) (*GenerateDataResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Generating data...", "ContextDir", args.ContextDir)

	randomByte := args.MaxFileSize - args.MinFileSize
	c := args.MinFileSize + math_rand.Intn(randomByte)

	data := make([]byte, c)
	_, err := rand.Read(data)
	if err != nil {
		return nil, err
	}
	fileName := filepath.Join(args.ContextDir, DataFile)
	_, err = saveToFile(fileName, data)
	if err != nil {
		logger.Error("generateActivity failed to save tmp file.", "Error", err, "ContextDir", args.ContextDir)
		return nil, err
	}

	logger.Info("generated data Activity succeed.", "WorkflowID", args.ContextDir)
	return &GenerateDataResult{}, nil
}
