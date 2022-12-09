package models

import (
	"encoding/json"
	"github.com/dustin/go-humanize"
	"github.com/urfave/cli/v2"
	"os"
	"plex-go-sync/internal/filesystem"
	"plex-go-sync/internal/logger"
	. "plex-go-sync/internal/structures"
	"strings"
	"time"
)

const bitrateFilter = 3500 * humanize.KByte
const heightFilter = 720
const widthFilter = 1280
const crfFilter = 23
const mediaFormat = "mp4"
const paddingBytes = 500 * humanize.MiByte

type Config struct {
	FastConvert       bool        `json:"-"`
	Server            string      `json:"sourceServer"`
	DestinationServer string      `json:"destinationServer"`
	Token             string      `json:"token"`
	Source            string      `json:"sourcePath"`
	Destination       string      `json:"destinationPath"`
	Playlists         []Playlist  `json:"playlists"`
	MediaFormat       MediaFormat `json:"mediaFormat"`
}

type Playlist struct {
	Clean   bool                     `json:"clean"`
	Name    string                   `json:"name"`
	RawSize string                   `json:"size"`
	Size    int64                    `json:"-"`
	Base    string                   `json:"-"`
	Items   OrderedMap[PlaylistItem] `json:"items"`
}

type MediaFormat struct {
	BitrateFilter int    `json:"bitrate"`
	HeightFilter  int    `json:"height"`
	WidthFilter   int    `json:"width"`
	CrfFilter     int    `json:"crf"` // deprecated
	Format        string `json:"format"`
}

func ReadConfig(ctx *cli.Context) (*Config, error) {
	path := ctx.Path("config")
	var config Config
	logger.LogInfo("Loading config...")
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return nil, err
	}

	if ctx.String("server") != "" {
		config.Server = ctx.String("server")
	}
	if ctx.String("token") != "" {
		config.Token = ctx.String("token")
	}
	if ctx.String("destination-server") != "" {
		config.DestinationServer = ctx.String("destination-server")
	}
	if ctx.StringSlice("playlist") != nil && len(ctx.StringSlice("playlist")) > 0 {
		for i := 0; i < len(ctx.StringSlice("playlist")); i++ {
			config.Playlists = append(config.Playlists, *NewPlaylist(ctx.StringSlice("playlist")[i], ctx.StringSlice("size")[i]))
		}
	}

	config.FastConvert = ctx.Bool("fast")

	if config.MediaFormat.Format == "" {
		config.MediaFormat.Format = mediaFormat
	}
	if config.MediaFormat.CrfFilter == 0 {
		config.MediaFormat.CrfFilter = crfFilter
	}
	if config.MediaFormat.BitrateFilter == 0 {
		config.MediaFormat.BitrateFilter = bitrateFilter
	}
	if config.MediaFormat.HeightFilter == 0 {
		config.MediaFormat.HeightFilter = heightFilter
	}
	if config.MediaFormat.WidthFilter == 0 {
		config.MediaFormat.WidthFilter = widthFilter
	}

	return &config, err
}

func NewPlaylist(name string, rawSize string) *Playlist {
	size, _ := humanize.ParseBytes(rawSize)
	return &Playlist{Name: name, RawSize: rawSize, Size: int64(size)}
}

// GetBase get the base directory of the playlist
func (p *Playlist) GetBase() string {
	if p.Base == "" {
		paths := p.Items.Front().Value.Paths
		p.Base, _, _ = strings.Cut(strings.TrimLeft(paths[len(paths)-1], "/"), "/")
	}
	return p.Base
}

func (p *Playlist) GetSize() int64 {
	if p.Size == 0 {
		size, _ := humanize.ParseBytes(p.RawSize)
		p.Size = int64(size)
	}
	return p.Size
}

// GetTotalSize get the size of the existing files in the playlist plus the remaining free space
// OR the size limit of the destination config
func (p *Playlist) GetTotalSize(root string, existing int64) int64 {
	if p.RawSize == "" {
		fs := filesystem.NewFileSystem(root)
		size, err := fs.GetFreeSpace(p.GetBase())
		if err != nil {
			return 0
		}
		// leave some space on the drive
		return int64(size) - paddingBytes + existing
	}
	size, _ := humanize.ParseBytes(p.RawSize)
	return int64(size)
}

type PlaylistItem struct {
	Paths    []string      `json:"paths"`
	Parent   string        `json:"parent"`
	Duration time.Duration `json:"duration"`
}

func (p PlaylistItem) GetSize(f filesystem.FileSystem) uint64 {
	for _, item := range p.Paths {
		size, err := f.GetSize(item)
		if err == nil {
			return size
		}
	}
	return 0
}

func (p PlaylistItem) GetDuration() time.Duration {
	return p.Duration
}
