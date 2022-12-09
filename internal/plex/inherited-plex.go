package plex

import "github.com/jrudio/go-plex-client"

type PlaylistContainer struct {
	MediaContainer PlaylistMediaContainer `json:"MediaContainer"`
}

type PlaylistMediaContainer struct {
	plex.MediaContainer
	Playlist []Playlist `json:"Playlist"`
}

type Playlist struct {
	RatingKey    int    `json:"ratingKey"`
	Key          string `json:"key"`
	GUID         string `json:"guid"`
	Title        string `json:"title"`
	Type         string `json:"type"`
	Summary      string `json:"summary"`
	Smart        int    `json:"smart"`
	PlaylistType string `json:"playlistType"`
	Composite    string `json:"composite"`
	Icon         string `json:"icon"`
	Content      string `json:"content"`
	Duration     int    `json:"duration"`
	LeafCount    int    `json:"leafCount"`
	AddedAt      int    `json:"addedAt"`
	UpdatedAt    int    `json:"updatedAt"`
}
