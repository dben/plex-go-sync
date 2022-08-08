package actions

import (
	"github.com/dustin/go-humanize"
	"math/rand"
	"path"
	"plex-go-sync/internal/filesystem"
	"plex-go-sync/internal/logger"
	"plex-go-sync/internal/plex"
	"strings"
	"time"
)

func CleanPlaylist(playlist *Playlist, config *Config, fs filesystem.FileSystem) (map[string]uint64, int64, string, error) {
	var playlistItems []PlaylistItem
	var playlistItemMap map[string]bool
	var err error
	if len(playlist.Items) == 0 {
		playlistItems, playlistItemMap, err = GetPlaylistItems(playlist.Name, config.Server, config.Token)
	} else {
		playlistItems = playlist.Items
		playlistItemMap = nil
	}
	playlist.Items = playlistItems
	if err != nil {
		return nil, 0, "", err
	}

	base, err := getBase(playlistItems)
	if err != nil {
		return nil, 0, base, err
	}

	// if we are cleaning the directory, we need to get all playlist items so that we don't delete anything that is
	// still in one of the playlists
	if playlistItemMap != nil && playlist.Clean {
		var dir = playlist.GetBase() // get the base directory of the playlist
		for altList := range config.Playlists {
			if config.Playlists[altList].Name != playlist.Name {
				logger.LogInfo("Getting playlist items from", config.Playlists[altList].Name, "so that we don't delete them")
				err := GetAltItems(config.Playlists[altList].Name, config.Server, config.Token, dir, playlistItemMap)
				if err != nil {
					return nil, 0, base, err
				}
			}
		}
	}

	// if the clean flag is not set, we clear the playlistItemMap so nothing is deleted
	if !playlist.Clean {
		playlistItemMap = nil
	}

	logger.LogInfo("Cleaning ", path.Join(fs.GetPath(), base), " directory")
	existing, existingSize, err := fs.Clean(base, playlistItemMap)
	if err == nil {
		logger.LogInfo("Existing  Size: ", humanize.Bytes(uint64(existingSize)))
	}

	return existing, existingSize, base, err
}

func GetAltItems(name string, server string, token string, baseDir string, itemMap map[string]bool) error {
	plexServer, err := plex.New(server, token)
	if err != nil {
		logger.LogError("Failed to connect to plex: ", err)
		return err
	}

	items, err := plexServer.GetPlaylistItems(name)
	if err != nil {
		return err
	}

	for _, item := range items.MediaContainer.Metadata {
		var mediaPath, key = plex.GetMediaPath(item, heightFilter)
		var base, _, _ = strings.Cut(strings.TrimLeft(mediaPath, "/"), "/")
		if base != baseDir {
			continue
		}
		itemMap[key] = true
	}
	return nil
}

func GetPlaylistItems(name string, server string, token string) ([]PlaylistItem, map[string]bool, error) {
	var results []PlaylistItem
	plexServer, err := plex.New(server, token)
	if err != nil {
		logger.LogError("Failed to connect to plex: ", err)
		return nil, nil, err
	}

	items, err := plexServer.GetPlaylistItems(name)
	if err != nil {
		return nil, nil, err
	}

	parents := make(map[string]bool)
	hashmap := make(map[string]bool)

	for _, item := range items.MediaContainer.Metadata {
		var mediaPath, key = plex.GetMediaPath(item, heightFilter)
		if mediaPath == "" {
			continue
		}

		newItem := PlaylistItem{Path: mediaPath, Parent: item.GrandparentTitle}
		results = append(results, newItem)
		parents[item.GrandparentTitle] = true
		hashmap[key] = true
	}

	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(results), func(i, j int) {
		results[i], results[j] = results[j], results[i]
	})
	if items.MediaContainer.Metadata[0].Type != "episode" {
		logger.LogVerbose("Playlist ", name, " retrieved, ", len(results), " items")
		return results, hashmap, err
	}

	// if we are dealing with a tv show playlist, we will sort the shuffled results to ensure we evenly pick from each show in the list.
	var i = 0
	for i < len(results) {
		for key := range parents {
			if results[i].Parent == key {
				i++
				continue
			}
			for j := i + 1; j < len(results); j++ {
				if results[j].Parent == key {
					results[i], results[j] = results[j], results[i]
					break
				}
			}
			if results[i].Parent == key {
				i++
				continue
			}
			logger.LogVerbose("End of ", key, " at ", i)
			delete(parents, key)
		}

	}

	logger.LogVerbose("Playlist ", name, " retrieved, ", len(results), " items")
	return results, hashmap, err
}
