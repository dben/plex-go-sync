package test

import (
	"context"
	"plex-go-sync/internal/ffmpeg"
	"plex-go-sync/internal/filesystem"
	"plex-go-sync/internal/models"
	"testing"
)

// TestHelloName calls greetings.Hello with a name, checking
// for a valid return value.
func TestFile(t *testing.T) {
	name := "test/test.mkv"
	ctx := context.WithValue(context.Background(), "config", &models.Config{
		MediaFormat: models.MediaFormat{
			BitrateFilter: 1000000,
			HeightFilter:  720,
		},
	})
	ok, duration, audio, err := ffmpeg.Probe(&ctx, filesystem.NewFileSystem(".").GetFile(name), 100)
	if err != nil {
		return
	}
	t.Log(ok, duration, audio, err)
	if !ok {
		t.Error("Not ok")
	}
}
