package clean

import (
	"context"
	client "github.com/jrudio/go-plex-client"
	"golang.org/x/exp/maps"
	"math/rand"
	"plex-go-sync/internal/logger"
	"plex-go-sync/internal/models"
	"plex-go-sync/internal/plex"
	. "plex-go-sync/internal/structures"
	"time"
)

func GetPlaylistItems(ctx *context.Context, name string) (*OrderedMap[models.PlaylistItem], error) {
	re := NewOrderedMap[models.PlaylistItem]()

	metadata, err := PopulateMediaItems(ctx, name, "", &re)
	if err != nil {
		return nil, err
	}

	rand.Seed(time.Now().UnixNano())
	re.Shuffle()

	if metadata[0].Type != "episode" {
		logger.LogInfo("Playlist ", name, " retrieved, ", re.Len(), " items")
		return &re, err
	}

	distinctParents := make(map[string]bool)
	for _, item := range metadata {
		distinctParents[item.GrandparentTitle] = true
	}
	parents := maps.Keys(distinctParents)

	i := -1
	// if we are dealing with a tv show playlist, we will sort the shuffled results to ensure we evenly pick from each show in the list.
itemLoop:
	for item := re.Front(); item != nil; item = item.Next() {
		i = i + 1
		if i >= len(parents) {
			i = 0
		}
		if parents[i] == item.Value.Parent {
			continue
		}

		for s := item.Next(); s != nil; s = s.Next() {
			if parents[i] == s.Value.Parent {
				re.Swap(s, item)
				item = s
				continue itemLoop
			}
		}

		parents = append(parents[:i], parents[i+1:]...)
		i = i - 1
	}

	logger.LogVerbose("Playlist ", name, " retrieved, ", re.Len(), " items")
	return &re, err
}

func PopulateMediaItems(ctx *context.Context, name string, baseDir string, itemMap *OrderedMap[models.PlaylistItem]) ([]client.Metadata, error) {
	config := models.GetConfig(ctx)
	plexServer, err := plex.New(config.Server, config.Token)
	if err != nil {
		logger.LogError("Failed to connect to plex: ", err)
		return nil, err
	}

	items, err := plexServer.GetPlaylistItems(name)
	if err != nil {
		return nil, err
	}

	for _, item := range items.MediaContainer.Metadata {
		var mediaPaths, duration = plex.GetMediaPath(ctx, item, baseDir)

		if len(mediaPaths) == 0 {
			continue
		}
		keys := make([]string, len(mediaPaths))
		for i, path := range mediaPaths {
			keys[i] = plex.GetKey(path)
		}

		newItem := models.PlaylistItem{Paths: mediaPaths, Parent: item.GrandparentTitle, Duration: duration}
		itemMap.SetAll(keys, newItem)
	}
	return items.MediaContainer.Metadata, nil
}
