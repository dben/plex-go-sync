package filesystem

import (
	"github.com/dustin/go-humanize"
	"github.com/shirou/gopsutil/disk"
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

func (f *LocalFileSystem) GetFreeSpace(base string) (uint64, error) {
	stat, err := disk.Usage(path.Join(f.Path, base))
	return stat.Free, err
}

func (f *LocalFileSystem) Clean(base string, lookup map[string]bool) (map[string]uint64, int64, error) {
	logger.LogInfo("Cleaning ", base)
	dir := f.abs(base)
	return cleanFiles(&osImpl{}, os.DirFS(dir), lookup)
}

func (f *LocalFileSystem) DownloadFile(fs FileSystem, filename string, id string) (uint64, error) {
	absPath := f.abs(filename)

	if stat, err := os.Stat(absPath); err == nil {
		return uint64(stat.Size()), nil
	}

	if err := os.MkdirAll(path.Dir(absPath), 0755); err != nil {
		return 0, err
	}
	file, err := os.Create(absPath)
	if err != nil {
		return 0, err
	}
	if _, err = file.Stat(); err != nil {
		return 0, err
	}

	return copyFile(fs.GetFile(filename), file, absPath, id)
}

func (f *LocalFileSystem) GetFile(filename string) File {
	return &FileImpl{Path: strings.TrimPrefix(filename, "/"), FileSystem: f}
}

func (f *LocalFileSystem) ReadFile(filename string) (io.ReadCloser, error) {
	logger.LogVerbose("Reading file", filename)
	return os.Open(f.abs(filename))
}

func (f *LocalFileSystem) FileWriter(filename string) (io.WriteCloser, error) {
	absPath := f.abs(filename)
	logger.LogVerbose("Creating directory ", path.Dir(absPath))
	if err := os.MkdirAll(path.Dir(absPath), 0755); err != nil {
		return nil, err
	}
	logger.LogVerbose(absPath)
	return os.Create(absPath)
}

func (f *LocalFileSystem) GetPath() string {
	return f.Path
}

func (f *LocalFileSystem) GetSize(filename string) uint64 {
	stat, err := os.Stat(f.abs(filename))
	if err != nil {
		logger.LogWarningf("Error getting size of file (%s/%s): %s\n", f.Path, filename, err.Error())
		return 0
	}
	logger.LogVerbosef("Size of file (%s/%s): %s\n", f.Path, filename, humanize.Bytes(uint64(stat.Size())))

	return uint64(stat.Size())
}

func (f *LocalFileSystem) IsLocal() bool {
	return true
}

func (f *LocalFileSystem) Mkdir(dir string) error {
	dir = f.abs(dir)
	logger.LogVerbose("Creating directory ", dir)
	return os.MkdirAll(dir, 0755)
}

func (f *LocalFileSystem) RemoveAll(dir string) error {
	dir = f.abs(dir)
	logger.LogVerbose("Removing directory ", dir)
	readDir, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, file := range readDir {
		err := os.RemoveAll(path.Join(dir, file.Name()))
		if err != nil {
			logger.LogWarning("Error removing file", file.Name(), ": ", err.Error())
		}
	}
	return nil
}

func (f *LocalFileSystem) Remove(dir string) error {
	dir = f.abs(dir)
	logger.LogVerbose("Removing file ", dir)
	return os.Remove(dir)
}

func (f *LocalFileSystem) abs(filepath string) string {
	return path.Clean(path.Join(f.Path, strings.TrimPrefix(filepath, "/")))
}
