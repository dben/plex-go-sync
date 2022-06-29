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
