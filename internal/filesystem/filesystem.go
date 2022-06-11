package filesystem

import (
	"io"
	"log"
	"strings"
)

type FileSystem interface {
	Clean(base string, lookup map[string]bool) (map[string]uint64, uint64, error)
	DownloadFile(fs FileSystem, filename string) (uint64, error)
	GetFile(filename string) File
	GetPath() string
	GetSize(filename string) uint64
	IsLocal() bool
	ReadFile(filename string) (io.Reader, func(), error)
	Remove(filename string) error
	RemoveAll(dir string) error
	Mkdir(dir string) error
}

func NewFileSystem(base string) FileSystem {
	if base == "" {
		log.Println("base is empty")
		return nil
	}
	if strings.HasPrefix(base, "smb://") || strings.HasPrefix(base, "//") {
		return NewSmbFileSystem(base)
	}
	return NewLocalFileSystem(base)
}