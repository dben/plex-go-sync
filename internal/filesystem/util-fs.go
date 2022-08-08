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

/**
 * Remove files which are not in a lookup map. If the lookup map is empty, nothing is removed.
 */
func cleanFiles(remove removeFS, fs iofs.FS, lookup map[string]bool) (map[string]uint64, int64, error) {
	results := make(map[string]uint64)
	totalSize := int64(0)
	err := iofs.WalkDir(fs, ".", func(path string, info iofs.DirEntry, err error) error {
		if info.IsDir() {
			return nil
		}
		// This isn't really used, but if the file's not a mp4 we don't care about it being completely correct
		key := path
		size := uint64(1) // skip this check if we can't look up size
		fi, err := info.Info()
		if err == nil {
			size = uint64(fi.Size())
		}

		split := strings.Split(path, ".")
		// If the file is a mp4, we need to calculate a valid key to compare against.
		// the key is the path to the file, without the base directory, extension, or a leading slash
		if len(split) > 1 && split[len(split)-1] == "mp4" {
			key = strings.Join(split[:len(split)-1], ".")
		}
		if (lookup == nil || lookup[key] == true) && size > 0 {
			detail, err := info.Info()
			if err != nil {
				return err
			}
			results[path] = uint64(detail.Size())
			totalSize += detail.Size()
			return nil
		}

		if err = remove.Remove(path); err != nil {
			logger.LogInfo("Removing: ", path, err.Error())
		} else {
			logger.LogInfo("Removing: ", path, humanize.Bytes(size), " key: ", key)
		}
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	return results, totalSize, nil
}

//goland:noinspection GoUnhandledErrorResult
func copyFile(from File, to io.WriteCloser, toPath string, id string) (uint64, error) {
	size := from.GetSize()
	logger.LogInfof("Copying %s \n", from.GetAbsolutePath())
	logger.LogInfof("     to %s \n", toPath)
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
		n, err := reader.Read(buff)
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
			}
			logger.ProgressClear(id + toPath)
			to.Close()
			break
		}
	}

	logger.LogVerbose(logger.Green+"Finished copying ", humanize.Bytes(currentSize), " to ", toPath, logger.Reset)
	return currentSize, err
}
