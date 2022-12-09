package sync

import (
	"context"
	"github.com/urfave/cli/v2"
	"plex-go-sync/internal/logger"
	"plex-go-sync/internal/models"
	"plex-go-sync/internal/plex"
)

func FromContext(c *cli.Context) error {
	logger.SetLogLevel(c.String("loglevel"))
	config, err := models.ReadConfig(c)

	if err != nil {
		logger.LogError(err.Error())
		return err
	}

	ctx := context.WithValue(c.Context, "config", config)
	err = SyncLibraries(&ctx)
	if err != nil {
		logger.LogError(err.Error())
	}

	return nil
}

func SyncLibrary(key string, source *plex.Server, dest *plex.Server) {
	destLibrary, err := dest.GetLibraryContent(key, "")
	if err != nil {
		logger.LogWarning("Skipping library: ", err.Error())
		return
	}
	srcLibrary, libType, err := source.GetLibrarySectionByName(destLibrary.MediaContainer.LibrarySectionTitle, "")
	if err != nil {
		logger.LogWarning("Skipping library: ", err.Error())
		return
	}

	if libType == "show" {
		for i, show := range destLibrary.MediaContainer.Metadata {
			progress := float64(i) / float64(len(destLibrary.MediaContainer.Metadata))
			logger.Progress("tv", progress)

			destEpisodes, err := dest.GetEpisodes(show.RatingKey)
			if err != nil {
				logger.LogWarning("Skipping show: ", err.Error())
				continue
			}
			srcShow := ""
			for _, srcItem := range srcLibrary.MediaContainer.Metadata {
				if srcItem.Title == show.Title {
					srcShow = srcItem.RatingKey
					break
				}
			}
			if srcShow == "" {
				continue
			}
			srcEpisodes, err := source.GetEpisodes(srcShow)
			if err != nil {
				logger.LogWarning("Skipping show: ", err.Error())
				continue
			}
			// TODO: optimize this
			for _, destEpisode := range destEpisodes.MediaContainer.Metadata {
				for _, srcEpisode := range srcEpisodes.MediaContainer.Metadata {
					if srcEpisode.ParentIndex == destEpisode.ParentIndex && srcEpisode.Index == destEpisode.Index {
						dest.SyncWatched(srcEpisode, destEpisode)
					}
				}
			}
		}
		logger.ProgressClear("tv")
	} else if libType == "movie" {
		for i, destMovie := range destLibrary.MediaContainer.Metadata {
			progress := float64(i) / float64(len(destLibrary.MediaContainer.Metadata))
			logger.Progress("movie", progress)

			for _, srcMovie := range srcLibrary.MediaContainer.Metadata {
				if srcMovie.Title == destMovie.Title {
					dest.SyncWatched(srcMovie, destMovie)
				}
			}
		}
		logger.ProgressClear("movie")
	}

}

func SyncLibraries(ctx *context.Context) error {
	config := models.GetConfig(ctx)
	source, err := plex.New(config.Server, config.Token)
	if err != nil {
		return err
	}

	dest, err := plex.New(config.DestinationServer, config.Token)
	if err != nil {
		return err
	}

	lib := dest.RefreshLibraries(ctx)
	for key := range lib {
		go SyncLibrary(key, source, dest)
	}
	return nil
}
