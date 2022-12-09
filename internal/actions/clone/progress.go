package clone

import (
	"encoding/json"
	"os"
	"plex-go-sync/internal/logger"
	"plex-go-sync/internal/models"
)

func WriteProgress(playlists []models.Playlist) error {
	file, err := os.Create("progress.json")
	if err != nil {
		logger.LogWarning("Error creating progress file: ", err)
		return err
	}
	defer func(file *os.File) {
		_ = file.Close()
	}(file)
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(playlists)
	if err != nil {
		logger.LogWarning("Error writing progress file: ", err)
	}
	err = file.Close()
	if err != nil {
		logger.LogWarning("Error writing progress file: ", err)
	}
	return err
}

func LoadProgress() ([]models.Playlist, error) {
	var playlists []models.Playlist
	file, err := os.Open("progress.json")
	if err != nil {
		logger.LogInfo("No progress file found")
		return playlists, err
	}
	defer func(file *os.File) {
		_ = file.Close()
	}(file)
	decoder := json.NewDecoder(file)
	if err = decoder.Decode(&playlists); err != nil {
		logger.LogWarning("Error reading progress file: ", err)
		return playlists, err
	}
	return playlists, nil
}

func ClearProgress() {
	_ = os.Remove("progress.json")
}

func WatchProgress(progress chan *models.Playlist, playlists *[]models.Playlist) {
	for {
		updated, more := <-progress
		for i := range *playlists {
			playlist := &(*playlists)[i]
			if updated != nil && playlist.Name == updated.Name {
				(*playlists)[i].Items = (*updated).Items
				break
			}
		}
		if more {
			logger.LogVerbose("Writing progress.json")
			_ = WriteProgress(*playlists)
		} else {
			break
		}
	}
}
