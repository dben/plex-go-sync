package filesystem

import (
	"fmt"
	"github.com/dustin/go-humanize"
	"io"
	iofs "io/fs"
	"plex-go-sync/internal/logger"
	"strings"
	"time"
)

func cleanFiles(remove removeFS, fs iofs.FS, lookup map[string]bool) (map[string]uint64, uint64, error) {
	results := make(map[string]uint64)
	totalSize := uint64(0)
	err := iofs.WalkDir(fs, ".", func(path string, info iofs.DirEntry, err error) error {
		if info.IsDir() {
			return nil
		}
		key := path
		size := uint64(1) // skip this check if we can't look up size
		fi, err := info.Info()
		if err == nil {
			size = uint64(fi.Size())
		}

		split := strings.Split(path, ".")
		if len(split) > 1 && split[len(split)-1] == "mp4" {
			key = strings.Join(split[:len(split)-1], ".")
		}
		if (lookup == nil || lookup[key] == true) && size > 0 {
			detail, err := info.Info()
			if err != nil {
				return err
			}
			results[path] = uint64(detail.Size())
			totalSize += uint64(detail.Size())
			return nil
		}

		if err = remove.Remove(path); err != nil {
			logger.LogInfo("Removing: ", path, err.Error())
		} else {
			logger.LogInfo("Removing: ", path, humanize.Bytes(size))
		}
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	return results, totalSize, nil
}

//goland:noinspection GoUnhandledErrorResult
func copyFile(from File, to io.ReaderFrom, toPath string) (uint64, error) {
	pReader, pWriter := io.Pipe()
	size := from.GetSize()
	logger.LogInfof("Copying %s \n", from.GetAbsolutePath())
	logger.LogInfof("     to %s \n", toPath)
	if size == 0 {
		return 0, fmt.Errorf("file is empty: %s", from.GetAbsolutePath())
	}

	start := time.Now()
	reader, cleanup, err := from.ReadFile()
	defer pReader.Close()
	defer cleanup()

	if err != nil {
		return 0, err
	}

	go func() {
		buff := make([]byte, 1024*1024)
		currentSize := uint64(0)
		for {
			n, err := reader.Read(buff)
			if n != 0 {
				currentSize += uint64(n)
				progress := float64(currentSize) / float64(size)
				remaining := time.Duration((float64(time.Since(start)) / float64(currentSize)) * float64(size-currentSize))
				logger.Progress(progress, humanize.Bytes(currentSize), " copied, ", remaining.Round(time.Second).String(), " remaining")
				pWriter.Write(buff[:n])
			}
			if err != nil {
				pWriter.Close()
				fmt.Println()
				return
			}
		}
	}()

	written, err := to.ReadFrom(pReader)
	if err != nil {
		return uint64(written), err
	}

	return uint64(written), nil
}
