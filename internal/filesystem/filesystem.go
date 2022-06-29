package filesystem

import (
	"io"
	"plex-go-sync/internal/logger"
	"strings"
)

type FileSystem interface {
	Clean(base string, lookup map[string]bool) (map[string]uint64, int64, error)
	DownloadFile(fs FileSystem, filename string, id string) (uint64, error)
	FileWriter(filename string) (io.WriteCloser, error)
	GetFile(filename string) File
	GetPath() string
	GetSize(filename string) uint64
	IsLocal() bool
	ReadFile(filename string) (io.ReadCloser, error)
	Remove(filename string) error
	RemoveAll(dir string) error
	Mkdir(dir string) error
	GetFreeSpace(base string) (uint64, error)
}

func NewFileSystem(base string) FileSystem {
	if base == "" {
		logger.LogError("base is empty")
		return nil
	}
	if strings.HasPrefix(base, "smb://") || strings.HasPrefix(base, "//") {
		return NewSmbFileSystem(base)
	}
	return NewLocalFileSystem(base)
}
