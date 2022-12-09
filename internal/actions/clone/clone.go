package clone

import (
	"context"
	"github.com/dustin/go-humanize"
	"github.com/urfave/cli/v2"
	"plex-go-sync/internal/actions/clean"
	"plex-go-sync/internal/actions/sync"
	. "plex-go-sync/internal/filesystem"
	"plex-go-sync/internal/logger"
	"plex-go-sync/internal/models"
	. "plex-go-sync/internal/structures"
	"time"
)

func FromContext(c *cli.Context) error {
	logger.SetLogLevel(c.String("loglevel"))
	config, err := models.ReadConfig(c)
	if err != nil {
		logger.LogError(err.Error())
		return err
	}
	ctx := context.WithValue(c.Context, "config", config)

	dest := NewFileSystem(config.Destination)
	src := NewFileSystem(config.Source)
	var wg WaitGroupCount
	progress := make(chan *models.Playlist, c.Int("threads"))

	if c.Bool("reset") {
		ClearProgress()
	} else if playlists, err := LoadProgress(); err == nil {
		config.Playlists = playlists
	}

	logger.LogInfo("Playlists to copy: ", len(config.Playlists))
	go WatchProgress(progress, &config.Playlists)

	logger.LogInfo("Throttling to", c.Int("threads"), "concurrent threads")
	for j := 0; j < len(config.Playlists); j++ {
		playlist := &config.Playlists[j]
		wg.Add(1)
		select {
		case <-wg.WaitFor(c.Int("threads")):
			go func() {
				FromPlaylist(&ctx, playlist, src, dest, progress)
				wg.Done()
			}()
		case <-c.Done():
			logger.LogWarning("Received interrupt signal, stopping", wg.GetCount())
			close(progress)
			CloseAllSmbConnections()
			return nil
		}

	}
	wg.Wait()
	close(progress)
	ClearProgress()
	CloseAllSmbConnections()
	if !models.IsDone(&c.Context) {
		err = sync.SyncLibraries(&ctx)
	}
	if err != nil {
		logger.LogError(err.Error())
	}
	return nil
}

func FromPlaylist(ctx *context.Context, playlist *models.Playlist, src FileSystem, dest FileSystem, progress chan<- *models.Playlist) {
	var config = models.GetConfig(ctx)

	existingFiles, existingSize, err := clean.FromPlaylist(ctx, playlist, dest)

	if err != nil {
		logger.LogWarning("Skipping playlist: ", err.Error())
		return
	}

	// totalBytes is the total allowed size of the playlist
	totalBytes := playlist.GetTotalSize(config.Destination, existingSize)

	// playlist.Size is the remaining size of the playlist after removing existing files
	playlist.Size = totalBytes - existingSize

	logger.LogInfo("Starting conversion")

	start := time.Now().Add(-time.Second)

	var i = 0
	// loop through each playlist item
mainLoop:
	for item := playlist.Items.Front(); item != nil; item = item.Next() {
		if models.IsDone(ctx) {
			playlist.Items = NewOrderedMap[models.PlaylistItem]()
			return
		}
		displayCloneProcessStatus(playlist, existingSize, totalBytes, start)

		// check if the file already exists
		for _, key := range item.Keys {
			if _, exists := existingFiles[key]; exists {
				logger.LogInfo("File exists, skipping ", item.Value.Paths[0])
				continue mainLoop
			}
		}

		// get the size of the original file
		originalBytes := int64(item.Value.GetSize(src))

		// if the remaining size available for the playlist is less than the size of the item, remove any extraneous
		// items from the dest directory until the size needed to copy the item is available
		if playlist.Size < originalBytes && i != playlist.Items.Len()-1 {
			logger.LogInfo("Making space - Checking if any of the", playlist.Items.Len()-i, "lower priority items exist")
			toRemove := originalBytes - playlist.Size
			clearedBytes := removeLast(ctx, toRemove, dest, item, playlist.Items.Back(), &existingFiles)
			if clearedBytes > 0 {
				logger.LogInfo("Removed last", humanize.Bytes(uint64(clearedBytes)), " from ", playlist.Name)
				playlist.Size += clearedBytes
				displayCloneProcessStatus(playlist, existingSize, totalBytes, start)
			}
		}

		var destFile File = nil
		if playlist.Size > humanize.MiByte*50 {
			var size uint64 = 0
			destFile, size, err = Transcode(ctx, playlist.Name, src, dest, item.Value)
			if err != nil {
				logger.LogErrorf("Error reencoding %s: %s\n", item.Value.Paths[0], err)
				if destFile != nil {
					_ = destFile.Remove()
				}
				continue mainLoop
			}
			playlist.Size = playlist.Size - int64(size)
		}

		// Are we done?
		if (humanize.MiByte * 50) >= uint64(playlist.Size) {
			if destFile != nil {
				_ = destFile.Remove()
			}
			logger.LogInfo(logger.Green, "Size limit reached.", logger.Reset)
			logger.ProgressClear(playlist.Name)
			playlist.Size = 0
			playlist.Items = NewOrderedMap[models.PlaylistItem]()
			break mainLoop
		}

		playlist.Size, totalBytes = checkFreeSpace(config.Destination, playlist.Size, playlist.GetBase(), totalBytes)

		progress <- playlist
		logger.LogVerbose("Moving to next item")
		i++
	}
	logger.LogInfo(logger.Green, "Finished copying ", playlist.Name, logger.Reset)
}
