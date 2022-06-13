package actions

import (
	"fmt"
	"github.com/dustin/go-humanize"
	"github.com/urfave/cli"
	"path"
	"plex-go-sync/internal/filesystem"
	"plex-go-sync/internal/logger"
	"plex-go-sync/internal/plex"
	"time"
)

func SyncPlayStatus(c *cli.Context) error {
	logger.SetLogLevel(c.String("log-level"))
	config, err := ReadConfig(c.String("configs"), c)
	if err != nil {
		logger.LogError(err.Error())
		return err
	}

	source, err := plex.New(config.Server, config.Token)
	dest, err := plex.New(config.DestinationServer, config.Token)

	lib := dest.RefreshLibraries()
	if err != nil {
		logger.LogError(err.Error())
		return err
	}
	for key := range lib {

		destLibrary, err := dest.GetLibraryContent(key, "")
		if err != nil {
			logger.LogWarning("Skipping library: ", err.Error())
			continue
		}
		srcLibrary, libType, err := source.GetLibrarySectionByName(destLibrary.MediaContainer.LibrarySectionTitle, "")
		if err != nil {
			logger.LogWarning("Skipping library: ", err.Error())
			continue
		}

		if libType == "show" {
			for i, show := range destLibrary.MediaContainer.Metadata {
				progress := float64(i) / float64(len(destLibrary.MediaContainer.Metadata))
				logger.Progress(progress)

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
		} else if libType == "movie" {
			for i, destMovie := range destLibrary.MediaContainer.Metadata {
				progress := float64(i) / float64(len(destLibrary.MediaContainer.Metadata))
				logger.Progress(progress)

				for _, srcMovie := range srcLibrary.MediaContainer.Metadata {
					if srcMovie.Title == destMovie.Title {
						dest.SyncWatched(srcMovie, destMovie)
					}
				}
			}
		}
	}
	fmt.Println()
	return nil
}

func CloneLibraries(c *cli.Context) error {
	logger.SetLogLevel(c.String("log-level"))
	config, err := ReadConfig(c.String("configs"), c)
	if err != nil {
		logger.LogError(err.Error())
		return err
	}
	dest := filesystem.NewFileSystem(config.Destination)
	src := filesystem.NewFileSystem(config.Source)

	playlists, err := LoadProgress()
	if err == nil {
		config.Playlists = playlists
	} else {
		playlists = config.Playlists
	}

	logger.LogInfo("Playlists to copy: ", len(config.Playlists))

	for j := 0; j < len(playlists); j++ {
		var playlistItems []PlaylistItem
		var base string
		var existing map[string]uint64
		var existingSize uint64

		if len(playlists[j].Items) == 0 {
			var playlistItemMap map[string]bool
			playlistItems, playlistItemMap, err = GetPlaylistItems(playlists[j].Name, config.Server, config.Token)
			playlists[j].Items = playlistItems
			base, err = getBase(playlistItems)
			logger.LogInfo("Cleaning ", path.Join(dest.GetPath(), base), " directory")
			existing, existingSize, err = dest.Clean(base, playlistItemMap)
		} else {
			playlistItems = playlists[j].Items
			base, err = getBase(playlistItems)
			existing, existingSize, err = dest.Clean(base, nil)
		}
		logger.LogInfo("Existing Size: ", humanize.Bytes(existingSize))

		if err != nil {
			logger.LogWarning("Skipping playlist: ", err.Error())
			continue
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

		totalBytes := playlists[j].GetTotalSize()
		playlists[j].Size = totalBytes - existingSize

		logger.LogInfo("Starting conversion")

		start := time.Now().Add(-time.Second)

		for i := 0; i < len(playlistItems); i++ {
			usedSize := totalBytes - playlists[j].Size + 1
			remaining := time.Duration((float64(time.Since(start)) / float64(usedSize-existingSize)) * float64(playlists[j].Size))
			if remaining < 0 { // this should be enough to check invalid times
				remaining = time.Duration(3*len(playlistItems)) * time.Minute // rough estimate
			}
			logger.LogInfof(logger.Green+"%s: %s used of %s, %s elapsed, %s (%s) remaining."+logger.Off+"\n", playlists[j].Name,
				humanize.Bytes(usedSize), playlists[j].RawSize,
				time.Since(start).Round(time.Second).String(), remaining.Round(time.Second).String(),
				humanize.Bytes(playlists[j].Size),
			)

			item := playlistItems[i]
			if existing[item.Path] > 0 {
				logger.LogInfo("File exists, skipping ", item.Path)
				continue
			}
			originalBytes := src.GetSize(item.Path)
			if playlists[j].Size < originalBytes && i != len(playlistItems)-1 {
				logger.LogInfo("Cleaning up old files...")
				clearedBytes := removeLast(originalBytes, dest, playlistItems[i+1:], existing)
				playlists[j].Size += clearedBytes
			}

			tmpFile, size, err := reencodeVideo(src, dest, item.Path, config)
			if err != nil {
				logger.LogErrorf("Error reencoding %s: %s\n", tmpFile.GetAbsolutePath(), err)
				if tmpFile != nil {
					_ = tmpFile.Remove()
				}
				continue
			}
			fits, err := ifItFitsItSits(tmpFile, dest, playlists[j].Size)
			if err != nil {
				logger.LogErrorf("Error moving %s: %s\n", tmpFile.GetAbsolutePath(), err)
				continue
			}
			if !fits {
				logger.LogInfo("Finished copying ", playlists[j].Name)
				playlists[j].Size = 0
				playlists[j].Items = []PlaylistItem{}
				break
			}

			playlists[j].Size -= size
			playlists[j].Items = playlistItems[i+1:]
			logger.LogInfo()

			_ = WriteProgress(config.Playlists)
		}
	}
	ClearProgress()
	return nil
}

func CleanLibrary(c *cli.Context) error {
	logger.SetLogLevel(c.String("log-level"))
	config, err := ReadConfig(c.String("configs"), c)
	if err != nil {
		logger.LogError(err.Error())
		return err
	}
	mediaLibrary := filesystem.NewFileSystem(config.Destination)

	for _, playlist := range config.Playlists {
		playlistItems, playlistItemMap, err := GetPlaylistItems(playlist.Name, config.Server, config.Token)
		if err != nil {
			logger.LogWarning("Skipping playlist", err.Error())
			continue
		}

		base, err := getBase(playlistItems)
		if err != nil {
			logger.LogWarning("Skipping playlist:", err.Error())
			continue
		}

		_, totalSize, err := mediaLibrary.Clean(base, playlistItemMap)
		if err != nil {
			logger.LogWarning("Skipping playlist:", err.Error())
			continue
		}

		logger.LogInfof("%s: %s used of %s\n", playlist.Name, humanize.Bytes(totalSize), humanize.Bytes(playlist.GetSize()))
	}
	return nil
}
