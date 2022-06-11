package actions

import (
	"encoding/json"
	"log"
	"os"
)

func WriteProgress(playlists []Playlist) error {
	file, err := os.Create("progress.json")
	if err != nil {
		log.Println("Error creating progress file:", err)
		return err
	}
	defer func(file *os.File) {
		_ = file.Close()
	}(file)
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err = encoder.Encode(playlists); err != nil {
		log.Println("Error writing progress file: ", err)
		return err
	}
	return nil
}

func LoadProgress() ([]Playlist, error) {
	var playlists []Playlist
	file, err := os.Open("progress.json")
	if err != nil {
		log.Println("No progress file found")
		return playlists, err
	}
	defer func(file *os.File) {
		_ = file.Close()
	}(file)
	decoder := json.NewDecoder(file)
	if err = decoder.Decode(&playlists); err != nil {
		log.Println("Error reading progress file: ", err)
		return playlists, err
	}
	return playlists, nil
}

func ClearProgress() {
	_ = os.Remove("progress.json")
}
