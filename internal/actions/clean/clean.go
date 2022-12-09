package clean

import (
	"context"
	"github.com/dustin/go-humanize"
	"github.com/urfave/cli/v2"
	"io/fs"
	"math"
	ospath "path"
	"plex-go-sync/internal/ffmpeg"
	"plex-go-sync/internal/filesystem"
	"plex-go-sync/internal/logger"
	"plex-go-sync/internal/models"
	. "plex-go-sync/internal/structures"
	"strings"
	"time"
)

func FromContext(c *cli.Context) error {
	logger.SetLogLevel(c.String("loglevel"))
	var wg WaitGroupCount
	config, err := models.ReadConfig(c)
	if err != nil {
		logger.LogError(err.Error())
		return err
	}
	ctx := context.WithValue(c.Context, "config", config)

	mediaLibrary := filesystem.NewFileSystem(config.Destination)

	logger.LogInfo("Throttling to", c.Int("threads"), "concurrent threads")
	for j := 0; j < len(config.Playlists); j++ {
		playlist := config.Playlists[j]
		wg.Add(1)
		select {
		case <-wg.WaitFor(c.Int("threads")):
			go func() {
				logger.LogInfo("Cleaning playlist", playlist.Name)
				_, usedSize, err := FromPlaylist(&ctx, &playlist, mediaLibrary)
				if err != nil {
					logger.LogWarning("Skipping playlist", err.Error())
				} else {
					logger.LogInfof(logger.Green+"%s: %s used of %s"+logger.Reset+"\n", playlist.Name, humanize.Bytes(uint64(usedSize)), playlist.RawSize)
				}
				logger.LogInfo("Finished cleaning playlist", playlist.Name)
				wg.Done()
			}()
		case <-c.Done():
			logger.LogWarning("Received interrupt signal, stopping", wg.GetCount())
			wg.Done()
			break
		}

	}
	wg.Wait()

	filesystem.CloseAllSmbConnections()
	return nil
}

func FromPlaylist(ctx *context.Context, playlist *models.Playlist, fs filesystem.FileSystem) (map[string]uint64, int64, error) {
	var config = models.GetConfig(ctx)
	items, err := GetPlaylistItems(ctx, playlist.Name)
	if err != nil {
		return nil, 0, err
	}

	playlist.Items = *items
	base := playlist.GetBase()
	itemsToKeep := playlist.Items.Copy()

	// if we are cleaning the directory, we need to get all playlist items so that we don't delete anything that is
	// still in one of the playlists
	if playlist.Items.Len() > 0 && playlist.Clean {
		var dir = playlist.GetBase() // get the base directory of the playlist
		for _, altList := range config.Playlists {
			if altList.Name != playlist.Name {
				logger.LogInfo("Getting playlist items from", altList.Name, "so that we don't delete them")
				_, err := PopulateMediaItems(ctx, altList.Name, dir, itemsToKeep)
				if err != nil {
					return nil, 0, err
				}
			}
		}
	}

	// if the clean flag is not set, we clear the playlistItemMap so nothing is deleted
	if !playlist.Clean {
		itemsToKeep = nil
	}

	logger.LogInfo("Cleaning ", ospath.Join(fs.GetPath(), base), " directory")
	existingFiles, existingSize, err := cleanFiles(ctx, fs, base, itemsToKeep)
	if err == nil {
		logger.LogInfo("Existing Size: ", humanize.Bytes(uint64(existingSize)))
	}

	return existingFiles, existingSize, err
}

/**
 * Remove files which are not in a lookup map. If the lookup map is empty, nothing is removed.
 */
func cleanFiles(ctx *context.Context, dir filesystem.FileSystem, base string,
	lookup *OrderedMap[models.PlaylistItem]) (map[string]uint64, int64, error) {

	totalSize := int64(0)
	var config = models.GetConfig(ctx)
	existingItems := make(map[string]uint64)
	iofs, err := dir.GetFileSystem(base)
	if err != nil {
		return nil, totalSize, err
	}

	directories := make([]string, 0)
	err = fs.WalkDir(iofs, ".", func(key string, info fs.DirEntry, err error) error {
		if models.IsDone(ctx) {
			return nil
		}

		if err != nil {
			logger.LogWarning(err.Error())
			return err
		}

		path := ospath.Join(base, key)

		if info.IsDir() {
			directories = append(directories, path)
			return nil
		}

		// We first read the size of the file
		fi, err := info.Info()
		size := uint64(1) // we skip this check if we can't look up size by setting it to 1
		if err == nil {
			size = uint64(fi.Size())
		}

		// If the file is a mp4, we need to calculate a valid key to compare against.
		// the key is the path to the file, without the base directory, extension, or a leading slash
		split := strings.Split(key, ".")

		if len(split) > 1 && split[len(split)-1] == config.MediaFormat.Format {
			key = strings.Join(split[:len(split)-1], ".")
		} else {
			logger.LogVerbose("Skipping file with extension", split[len(split)-1])
			removeItem(dir, path, size, 0)
			return nil
		}

		// if the lookup map is not empty, we remove any files that are not in the map
		var item models.PlaylistItem
		ok := false
		if lookup != nil {
			item, ok = lookup.Get(key)
			if !ok {
				logger.LogVerbose("Removing file", key, "because it is not in the lookup map")
				removeItem(dir, path, size, 0)
			}
		}

		// Now we need to check the duration of the file to see if it matches the duration of the playlist item
		file := dir.GetFile(path)
		duration := time.Duration(0)
		if err != nil {
			logger.LogVerbose("Error reading file: ", path, err.Error())
			removeItem(dir, path, size, duration)
			return nil
		}
		ok, duration, _, err = ffmpeg.Probe(ctx, file, size)
		if err != nil {
			logger.LogVerbose("Error parsing file: ", path, err.Error())
			return nil
		}

		if !ok && !config.FastConvert {
			logger.LogVerbose(" Existing file is incorrect format: ", path)
			removeItem(dir, path, size, duration)
			return nil
		}

		if duration > 0 && duration.Seconds() < 10 {
			if config.FastConvert {
				duration = time.Hour * 24
			} else {
				duration, err = ffmpeg.ProbeActualDuration(ctx, file)
				if err != nil {
					logger.LogVerbose("Error parsing file duration: ", path, err.Error())
					return nil
				}
			}
		}

		// If the lookup map is empty, we just remove any files that are empty or have a duration of 0
		if lookup == nil {
			if duration > 0 && size > 0 {
				existingItems[key] = size
				totalSize += int64(size)
				return nil
			}

			// Otherwise we remove any files that don't match the duration of the playlist item
		} else {
			if duration+time.Minute > item.GetDuration() && size > 0 {
				existingItems[key] = size
				totalSize += int64(size)
				logger.LogVerbose("Found existing file: ", path, " size: ", humanize.Bytes(existingItems[key]), duration.Round(time.Second))
				return nil
			}
		}

		removeItem(dir, path, size, duration)
		return nil
	})
	if err != nil {
		return nil, 0, err
	}

	// Now we need to remove any empty directories
	for i := len(directories) - 1; i >= 0; i-- {
		if dir.IsEmptyDir(directories[i]) {
			logger.LogInfo("Removing empty directory: ", directories[i])
			if err := dir.Remove(directories[i]); err != nil {
				logger.LogInfo(err.Error())
			}
		}
	}

	return existingItems, totalSize, nil
}

func removeItem(dir filesystem.FileSystem, path string, size uint64, duration time.Duration) {
	if err := dir.Remove(path); err != nil {
		logger.LogInfo("Removing: ", path, humanize.Bytes(size), math.Ceil(duration.Minutes()), "min")
		logger.LogInfo(err.Error())
	} else {
		logger.LogInfo("Removing: ", path, humanize.Bytes(size), math.Ceil(duration.Minutes()), "min")
	}
}
