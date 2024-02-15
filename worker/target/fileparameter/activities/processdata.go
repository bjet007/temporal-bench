package activities

import (
	"bytes"
	"context"
	"crypto/sha256"
	"io"
	math_rand "math/rand"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/pkg/errors"
	"go.temporal.io/sdk/activity"
)

type ProcessDataParams struct {
	ContextDir string
	ResultDir  string
	Duration   time.Duration
	CoresCount int
	Percentage int
}

type ProcessDataResult struct {
}

// ProcessData is a stub function to processes a file.
func ProcessData(ctx context.Context, args ProcessDataParams) (*ProcessDataResult, error) {
	logger := activity.GetLogger(ctx)
	// Read from file
	logger.Info("Processing data...", "ContextDir", args.ContextDir)
	dataFile := filepath.Join(args.ContextDir, DataFile)
	data, err := os.ReadFile(dataFile)
	if err != nil {
		return nil, err
	}
	logger.Info("Starting CPU Intensive Process...", "duration", args.Duration.String(), "ContextDir", args.ContextDir)
	runCPULoad(args.CoresCount, args.Duration, args.Percentage)
	logger.Info("CPU Intensive Process Completed, saving result...", "ResultDir", args.ResultDir)

	// Calculate checksum
	h := sha256.New()
	if _, err = io.Copy(h, bytes.NewReader(data)); err != nil {
		return nil, err
	}
	checksum := h.Sum(nil)

	fileName := filepath.Join(args.ResultDir, ChecksumFile)
	_, err = saveToFile(fileName, checksum)
	if err != nil {
		logger.Error("generateActivity failed to save tmp file.", "Error", err, "ResultDir", args.ResultDir)
		return nil, err
	}
	math_rand.Intn(100)
	if math_rand.Intn(100) < 10 {
		return nil, errors.New("Process Crash")
	}
	return &ProcessDataResult{}, nil
}

// RunCPULoad run CPU load in specify cores count and percentage
// Taken from https://github.com/vikyd/go-cpu-load
func runCPULoad(coresCount int, duration time.Duration, percentage int) {
	runtime.GOMAXPROCS(coresCount)

	// second     ,s  * 1
	// millisecond,ms * 1000
	// microsecond,μs * 1000 * 1000
	// nanosecond ,ns * 1000 * 1000 * 1000

	// every loop : run + sleep = 1 unit

	// 1 unit = 100 ms may be the best
	unitHundresOfMicrosecond := 1000
	runMicrosecond := unitHundresOfMicrosecond * percentage
	sleepMicrosecond := unitHundresOfMicrosecond*100 - runMicrosecond
	var wg sync.WaitGroup

	for i := 0; i < coresCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), duration)
			defer cancel()
			runtime.LockOSThread()
			// endless loop
			for {
				select {
				case <-ctx.Done():
					return
				default:
					begin := time.Now()
					for {
						// run 100%
						if time.Now().Sub(begin) > time.Duration(runMicrosecond)*time.Microsecond {
							break
						}
					}
					// sleep
					time.Sleep(time.Duration(sleepMicrosecond) * time.Microsecond)
				}

			}
		}()
	}
	wg.Wait()
}
