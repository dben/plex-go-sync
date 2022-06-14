package actions

import (
	"github.com/dustin/go-humanize"
	"github.com/urfave/cli"
	"plex-go-sync/internal/filesystem"
	"plex-go-sync/internal/logger"
	"sync"
)

func SyncPlayStatus(c *cli.Context) error {
	logger.SetLogLevel(c.String("loglevel"))
	config, err := ReadConfig(c.String("configs"), c)
	if err != nil {
		logger.LogError(err.Error())
		return err
	}

	err = SyncLibraries(config)
	if err != nil {
		logger.LogError(err.Error())
	}

	return nil
}

func CloneLibraries(c *cli.Context) error {
	logger.SetLogLevel(c.String("loglevel"))
	config, err := ReadConfig(c.String("configs"), c)
	if err != nil {
		logger.LogError(err.Error())
		return err
	}
	dest := filesystem.NewFileSystem(config.Destination)
	src := filesystem.NewFileSystem(config.Source)
	var wg sync.WaitGroup
	progress := make(chan *Playlist, 2)

	playlists, err := LoadProgress()
	if err == nil {
		config.Playlists = playlists
	} else {
		playlists = config.Playlists
	}

	logger.LogInfo("Playlists to copy: ", len(config.Playlists))
	go func() {
		for {
			_, more := <-progress
			logger.LogVerbose("Writing progress.json")
			_ = WriteProgress(config.Playlists)
			if !more {
				break
			}
		}
	}()

	for j := 0; j < len(playlists); j++ {
		wg.Add(1)
		go CloneLibrary(&playlists[j], config, src, dest, &wg, progress)
	}
	wg.Wait()
	close(progress)
	ClearProgress()
	filesystem.CloseAllSmbConnections()
	err = SyncLibraries(config)
	if err != nil {
		logger.LogError(err.Error())
	}
	return nil
}

func CleanLibrary(c *cli.Context) error {
	logger.SetLogLevel(c.String("loglevel"))
	config, err := ReadConfig(c.String("configs"), c)
	if err != nil {
		logger.LogError(err.Error())
		return err
	}
	mediaLibrary := filesystem.NewFileSystem(config.Destination)

	for _, playlist := range config.Playlists {
		_, usedSize, _, err := CleanPlaylist(&playlist, config, mediaLibrary)
		if err != nil {
			logger.LogWarning("Skipping playlist", err.Error())
			continue
		}

		logger.LogInfof(logger.Green+"%s: %s used of %s"+logger.DefaultColor+"\n", playlist.Name, humanize.Bytes(usedSize), playlist.RawSize)
	}

	filesystem.CloseAllSmbConnections()
	return nil
}
