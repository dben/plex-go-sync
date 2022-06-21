package actions

import (
	"github.com/dustin/go-humanize"
	"github.com/urfave/cli/v2"
	"plex-go-sync/internal/filesystem"
	"plex-go-sync/internal/logger"
	"sync"
)

func SyncPlayStatus(c *cli.Context) error {
	logger.SetLogLevel(c.String("loglevel"))
	config, err := ReadConfig(c.Path("config"), c)
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
	config, err := ReadConfig(c.Path("config"), c)
	if err != nil {
		logger.LogError(err.Error())
		return err
	}
	dest := filesystem.NewFileSystem(config.Destination)
	src := filesystem.NewFileSystem(config.Source)
	var wg sync.WaitGroup
	progress := make(chan *Playlist, 2)

	var playlists []Playlist
	if c.Bool("reset") {
		ClearProgress()
		playlists = config.Playlists
	} else if playlists, err = LoadProgress(); err == nil {
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
		go CloneLibrary(&playlists[j], config, src, dest, c.Bool("fast"), &wg, progress)
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
	config, err := ReadConfig(c.Path("config"), c)
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

		logger.LogInfof(logger.Green+"%s: %s used of %s"+logger.Reset+"\n", playlist.Name, humanize.Bytes(usedSize), playlist.RawSize)
	}

	filesystem.CloseAllSmbConnections()
	return nil
}
