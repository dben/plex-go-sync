package actions

import (
	"github.com/dustin/go-humanize"
	"math"
	"path"
	"plex-go-sync/internal/filesystem"
	"plex-go-sync/internal/logger"
	"strings"
	"sync"
	"time"
)

func CloneLibrary(playlist *Playlist, config *Config, src filesystem.FileSystem, dest filesystem.FileSystem, fast bool, wg *sync.WaitGroup, progress chan<- *Playlist) {
	existing, existingSize, base, err := CleanPlaylist(playlist, config, dest)
	playlistItems := playlist.Items

	if err != nil {
		logger.LogWarning("Skipping playlist: ", err.Error())
		wg.Done()
		return
	}

	logger.LogInfo("Cleaning ", path.Join(config.TempDir, "src", base), " directory")
	if config.TempDir != "" {
		if err = filesystem.NewLocalFileSystem(config.TempDir).RemoveAll(path.Join("src", base)); err != nil {
			logger.LogWarning(err.Error())
		}
	}

	logger.LogInfo("Cleaning ", path.Join(config.TempDir, "dest", base), " directory")
	if config.TempDir != "" {
		if err = filesystem.NewLocalFileSystem(config.TempDir).RemoveAll(path.Join("src", base)); err != nil {
			logger.LogWarning(err.Error())
		}
	}

	totalBytes := playlist.GetTotalSize(config.Destination, existingSize)
	playlist.Size = totalBytes - existingSize

	logger.LogInfo("Starting conversion")

	start := time.Now().Add(-time.Second)

	for i := 0; i < len(playlistItems); i++ {
		cloneStatus(*playlist, existingSize, totalBytes, start)

		item := playlistItems[i]
		_, key, _ := strings.Cut(strings.TrimPrefix(item.Path, "/"), "/")
		var originalBytes int64
		if existing[key] > 0 {
			originalBytes = 0
		} else {
			originalBytes = int64(src.GetSize(item.Path))
		}
		if playlist.Size < originalBytes && i != len(playlistItems)-1 {
			logger.LogInfo("Cleaning up old files...")
			toRemove := originalBytes - playlist.Size
			clearedBytes := removeLast(toRemove, dest, playlistItems[i+1:], existing)
			if clearedBytes > 0 {
				logger.LogInfo("Removed last", humanize.Bytes(uint64(clearedBytes)), " from ", playlist.Name)
				playlist.Size += clearedBytes
				cloneStatus(*playlist, existingSize, totalBytes, start)
			}
		}
		if existing[key] > 0 {
			logger.LogInfo("File exists, skipping ", item.Path)
			continue
		}

		if playlist.Size < humanize.MiByte*50 {
			logger.LogInfo(logger.Green, "Finished copying ", playlist.Name, logger.Reset)
			playlist.Size = 0
			playlist.Items = []PlaylistItem{}
			break
		}

		destFile, size, err := reencodeVideo(playlist.Name, src, dest, item.Path, config.TempDir, fast)
		if err != nil {
			logger.LogErrorf("Error reencoding %s: %s\n", item.Path, err)
			continue
		}

		if int64(size) > playlist.Size {
			_ = destFile.Remove()
			logger.LogInfo(logger.Green, "Finished copying ", playlist.Name, logger.Reset)
			playlist.Size = 0
			playlist.Items = []PlaylistItem{}
			break
		}

		playlist.Size = int64(math.Max(float64(playlist.Size)-float64(size), 0))
		playlist.Items = playlistItems[i+1:]

		progress <- playlist
		logger.LogVerbose("Moving to next item")
	}
	wg.Done()
}

func cloneStatus(playlist Playlist, existingSize int64, totalBytes int64, start time.Time) {
	if (logger.LogLevel != "WARN") && (logger.LogLevel != "ERROR") {
		usedSize := totalBytes - playlist.Size + 1
		remaining := time.Duration((float64(time.Since(start)) / float64(usedSize-existingSize)) * float64(playlist.Size))
		if remaining < 0 { // this should be enough to check invalid times
			remaining = time.Duration(3*len(playlist.Items)) * time.Minute // rough estimate
		}

		logger.LogPersistent(playlist.Name, logger.Green, playlist.Name, ": ",
			humanize.Bytes(uint64(usedSize)), " used of ", humanize.Bytes(uint64(totalBytes)), ", ",
			time.Since(start).Round(time.Second).String(), " elapsed, ",
			remaining.Round(time.Second).String(), " (", humanizeNegBytes(playlist.Size), ") remaining.",
			logger.Reset, "\n")
	}
}
