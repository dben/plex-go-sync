package actions

import (
	"github.com/dustin/go-humanize"
	"plex-go-sync/internal/filesystem"
	"strings"
)

type Config struct {
	TempDir           string     `json:"tempDir"`
	Server            string     `json:"sourceServer"`
	DestinationServer string     `json:"destinationServer"`
	Token             string     `json:"token"`
	Source            string     `json:"sourcePath"`
	Destination       string     `json:"destinationPath"`
	Playlists         []Playlist `json:"playlists"`
}

type Playlist struct {
	Clean     bool           `json:clean`
	LibraryId int            `json:"library"`
	Name      string         `json:"name"`
	RawSize   string         `json:"size"`
	Size      int64          `json:"bytes"`
	Items     []PlaylistItem `json:"items"`
}

func NewPlaylist(name string, rawSize string) *Playlist {
	size, _ := humanize.ParseBytes(rawSize)
	return &Playlist{Name: name, RawSize: rawSize, Size: int64(size)}
}

func (p *Playlist) GetBase() string {
	before, _, _ := strings.Cut(strings.TrimLeft(p.Items[0].Path, "/"), "/")
	return before
}

func (p *Playlist) GetSize() int64 {
	if p.Size == 0 {
		size, _ := humanize.ParseBytes(p.RawSize)
		p.Size = int64(size)
	}
	return p.Size
}

func (p *Playlist) GetTotalSize(root string, existing int64) int64 {
	if p.RawSize == "" {
		fs := filesystem.NewFileSystem(root)
		size, err := fs.GetFreeSpace(p.GetBase())
		if err != nil {
			return 0
		}
		// leave some space on the drive
		return int64(size) - (500 * humanize.MiByte) + existing
	}
	size, _ := humanize.ParseBytes(p.RawSize)
	return int64(size)
}

type PlaylistItem struct {
	Path   string `json:"path"`
	Parent string `json:"parent"`
}

func (p *PlaylistItem) GetSize(f filesystem.FileSystem) uint64 {
	return f.GetSize(p.Path)
}
