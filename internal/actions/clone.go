package actions

import (
	"github.com/dustin/go-humanize"
	"path"
	"plex-go-sync/internal/filesystem"
	"plex-go-sync/internal/logger"
	"sync"
	"time"
)

func CloneLibrary(playlist *Playlist, config *Config, src filesystem.FileSystem, dest filesystem.FileSystem, wg *sync.WaitGroup, progress chan<- *Playlist) {
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

	totalBytes := playlist.GetTotalSize()
	playlist.Size = totalBytes - existingSize

	logger.LogInfo("Starting conversion")

	start := time.Now().Add(-time.Second)

	for i := 0; i < len(playlistItems); i++ {
		usedSize := totalBytes - playlist.Size + 1
		remaining := time.Duration((float64(time.Since(start)) / float64(usedSize-existingSize)) * float64(playlist.Size))
		if remaining < 0 { // this should be enough to check invalid times
			remaining = time.Duration(3*len(playlistItems)) * time.Minute // rough estimate
		}
		if (logger.LogLevel != "WARN") && (logger.LogLevel != "ERROR") {
			logger.LogPersistent(playlist.Name, logger.Green, playlist.Name, ": ",
				humanize.Bytes(usedSize), " used of ", playlist.RawSize, ", ",
				time.Since(start).Round(time.Second).String(), " elapsed, ",
				remaining.Round(time.Second).String(), " (", humanize.Bytes(playlist.Size), ") remaining.",
				logger.DefaultColor, "\n")
		}

		item := playlistItems[i]
		if existing[item.Path] > 0 {
			logger.LogInfo("File exists, skipping ", item.Path)
			continue
		}
		originalBytes := src.GetSize(item.Path)
		if playlist.Size < originalBytes && i != len(playlistItems)-1 {
			logger.LogInfo("Cleaning up old files...")
			clearedBytes := removeLast(originalBytes, dest, playlistItems[i+1:], existing)
			playlist.Size += clearedBytes
		}

		tmpFile, size, err := reencodeVideo(playlist.Name, src, dest, item.Path, config)
		if err != nil {
			logger.LogErrorf("Error reencoding %s: %s\n", tmpFile.GetAbsolutePath(), err)
			if tmpFile != nil {
				_ = tmpFile.Remove()
			}
			continue
		}
		fits, err := ifItFitsItSits(playlist.Name, tmpFile, dest, playlist.Size)
		if err != nil {
			logger.LogErrorf("Error moving %s: %s\n", tmpFile.GetAbsolutePath(), err)
			continue
		}
		if !fits {
			logger.LogInfo("Finished copying ", playlist.Name)
			playlist.Size = 0
			playlist.Items = []PlaylistItem{}
			break
		}

		playlist.Size -= size
		playlist.Items = playlistItems[i+1:]

		progress <- playlist
		logger.LogVerbose("Moving to next item")
	}
	wg.Done()
}
