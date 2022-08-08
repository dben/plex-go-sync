package actions

import (
	"github.com/dustin/go-humanize"
	"path"
	"plex-go-sync/internal/filesystem"
	"plex-go-sync/internal/logger"
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

	var dir = playlist.GetBase() // get the base directory of the playlist
	for altList := range config.Playlists {
		if config.Playlists[altList].Name != playlist.Name {
			logger.LogInfo("Adding playlist items from ", config.Playlists[altList].Name, " so that we don't delete them")
			err := GetAltItems(config.Playlists[altList].Name, config.Server, config.Token, dir, playlistItemMap)
			if err != nil {
				return nil, 0, base, err
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
		logger.LogInfo("Existing Size: ", humanize.Bytes(uint64(existingSize)))
	}

	return existing, existingSize, base, err
}
