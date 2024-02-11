package activities

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.temporal.io/sdk/activity"
)

/**
 * Sample activities used by file processing sample workflow.
 */

const ChecksumFile = "checksum.txt"
const DataFile = "data.txt"

type BlobStore struct{}

func (b *BlobStore) uploadFile(ctx context.Context, workflowID string, filename string) (*string, error) {
	// dummy uploader
	_, err := os.ReadFile(filename)
	for i := 0; i < 5; i++ {
		time.Sleep(1 * time.Second)
		// Demonstrates that heartbeat accepts progress data.
		// In case of a heartbeat timeout it is included into the error.
		activity.RecordHeartbeat(ctx, i)
	}
	if err != nil {
		return nil, err
	}
	objectFile := fmt.Sprintf("s3://%s/%s", workflowID, filename)
	return &objectFile, nil
}

func saveToFile(fileName string, data []byte) (f *os.File, err error) {

	tmpFile, err := os.Create(fileName)
	if err != nil {
		return nil, err
	}
	_, err = tmpFile.Write(data)
	if err != nil {
		_ = os.Remove(tmpFile.Name())
		return nil, err
	}

	return tmpFile, nil
}
