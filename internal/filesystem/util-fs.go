package filesystem

import (
	"context"
	"fmt"
	"github.com/dustin/go-humanize"
	"io"
	"plex-go-sync/internal/logger"
	"time"
)

//goland:noinspection GoUnhandledErrorResult
func copyFile(ctx *context.Context, from File, to io.WriteCloser, toPath string, id string) (uint64, error) {
	done := (*ctx).Done()
	size, err := from.GetSize()
	logger.LogInfof("Copying %s \n", from.GetAbsolutePath())
	logger.LogInfof("     to %s \n", toPath)
	if err != nil {
		return 0, err
	}
	if size == 0 {
		return 0, fmt.Errorf("file is empty: %s", from.GetAbsolutePath())
	}

	start := time.Now()
	reader, err := from.ReadFile()
	defer reader.Close()

	if err != nil {
		logger.LogError(err.Error())
		return 0, err
	}

	buff := make([]byte, 4*1024*1024)
	currentSize := uint64(0)
	for {
		var n = 0
		select {
		case <-done:
			reader.Close()
			to.Close()

			return 0, fmt.Errorf("copying file %s was cancelled", from.GetAbsolutePath())
		default:
			n, err = reader.Read(buff)
		}
		if n != 0 {
			currentSize += uint64(n)
			progress := float64(currentSize) / float64(size)
			remaining := time.Duration((float64(time.Since(start)) / float64(currentSize)) * float64(size-currentSize))
			logger.Progress(id+toPath, progress, humanize.Bytes(currentSize), " copied, ", remaining.Round(time.Second).String(), " remaining")
			_, err2 := to.Write(buff[:n])
			if err2 != nil {
				logger.LogError(err2.Error())
			}
		}
		if err != nil {
			if err != io.EOF {
				logger.LogError(err.Error())
			} else {
				err = nil
			}
			logger.ProgressClear(id + toPath)
			to.Close()
			break
		}
	}

	logger.LogVerbose(logger.Green+"Finished copying ", humanize.Bytes(currentSize), " to ", toPath, logger.Reset)
	return currentSize, err
}
