package filesystem

import (
	"io"
	"os"
	"path"
	"plex-go-sync/internal/logger"
	"strings"
)

type LocalFileSystem struct {
	Path string
}

func NewLocalFileSystem(dir string) FileSystem {
	return &LocalFileSystem{Path: dir}
}
func (f *LocalFileSystem) Clean(base string, lookup map[string]bool) (map[string]uint64, uint64, error) {
	logger.LogInfo("Cleaning ", base)
	dir := path.Join(f.Path, strings.TrimPrefix(base, "/"))
	return cleanFiles(&osImpl{}, os.DirFS(dir), lookup)
}

func (f *LocalFileSystem) DownloadFile(fs FileSystem, filename string) (uint64, error) {
	absPath := path.Join(f.Path, strings.TrimPrefix(filename, "/"))
	logger.LogVerbose("Copying file from ", fs.GetPath(), " to ", absPath)

	if stat, err := os.Stat(absPath); err == nil {
		return uint64(stat.Size()), nil
	}

	if err := os.MkdirAll(path.Dir(absPath), 0755); err != nil {
		return 0, err
	}
	file, err := os.Create(path.Clean(absPath))
	if err != nil {
		return 0, err
	}

	return copyFile(fs.GetFile(filename), file, absPath)

}

func (f *LocalFileSystem) GetFile(filename string) File {
	return &FileImpl{Path: strings.TrimPrefix(filename, "/"), FileSystem: f}
}

func (f *LocalFileSystem) ReadFile(filename string) (io.Reader, func(), error) {
	logger.LogVerbose("Reading file", filename)
	file, err := os.Open(path.Clean(path.Join(f.Path, strings.TrimPrefix(filename, "/"))))
	if err != nil {
		logger.LogInfo("Error opening file: ", f.Path, filename)
	}
	return file, func() { _ = file.Close() }, err
}

func (f *LocalFileSystem) GetPath() string {
	return f.Path
}

func (f *LocalFileSystem) GetSize(filename string) uint64 {
	stat, err := os.Stat(path.Join(f.Path, strings.TrimPrefix(filename, "/")))
	if err != nil {
		logger.LogErrorf("Error getting size of file (%s/%s): %s\n", f.Path, filename, err.Error())
		return 0
	}
	logger.LogVerbosef("Size of file (%s/%s): %s\n", f.Path, filename, stat.Size())

	return uint64(stat.Size())
}

func (f *LocalFileSystem) IsLocal() bool {
	return true
}

func (f *LocalFileSystem) Mkdir(dir string) error {
	dir = path.Clean(path.Join(f.Path, strings.TrimPrefix(dir, "/")))
	logger.LogVerbose("Creating directory ", dir)
	return os.MkdirAll(dir, 0755)
}

func (f *LocalFileSystem) RemoveAll(dir string) error {
	dir = path.Clean(path.Join(f.Path, strings.TrimPrefix(dir, "/")))
	logger.LogVerbose("Removing directory ", dir)
	return os.RemoveAll(dir)
}

func (f *LocalFileSystem) Remove(dir string) error {
	dir = path.Clean(path.Join(f.Path, strings.TrimPrefix(dir, "/")))
	logger.LogVerbose("Removing file ", dir)
	return os.Remove(dir)
}
