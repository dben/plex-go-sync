package filesystem

import (
	"io"
	"path"
	"plex-go-sync/internal/logger"
)

type File interface {
	CopyFrom(fs FileSystem, id string) (uint64, error)
	CopyTo(fs FileSystem, id string) (File, error)
	MoveFrom(fs FileSystem, id string) (uint64, error)
	MoveTo(fs FileSystem, id string) (File, error)
	ReadFile() (io.ReadCloser, error)
	FileWriter() (io.WriteCloser, error)
	GetRelativePath() string
	GetAbsolutePath() string
	GetSize() uint64
	GetFileSystem() FileSystem
	IsLocal() bool
	Remove() error
}
type FileImpl struct {
	FileSystem FileSystem
	Path       string
	CachedSize uint64
}

func (f *FileImpl) CopyFrom(src FileSystem, id string) (uint64, error) {
	size, err := f.FileSystem.DownloadFile(src, f.Path, id)
	f.CachedSize = size
	return size, err
}
func (f *FileImpl) CopyTo(dest FileSystem, id string) (File, error) {
	destFile := &FileImpl{
		FileSystem: dest,
		Path:       f.Path,
	}
	size, err := destFile.CopyFrom(f.FileSystem, id)
	destFile.CachedSize = size
	return destFile, err
}
func (f *FileImpl) GetFileSystem() FileSystem {
	return f.FileSystem
}
func (f *FileImpl) ReadFile() (io.ReadCloser, error) {
	return f.FileSystem.ReadFile(f.Path)
}
func (f *FileImpl) FileWriter() (io.WriteCloser, error) {
	return f.FileSystem.FileWriter(f.Path)
}
func (f *FileImpl) GetRelativePath() string {
	return f.Path
}
func (f *FileImpl) GetAbsolutePath() string {
	return path.Join(f.FileSystem.GetPath(), f.Path)
}

func (f *FileImpl) GetSize() uint64 {
	if f.CachedSize == 0 {
		f.CachedSize = f.FileSystem.GetSize(f.Path)
	}
	return f.CachedSize
}
func (f *FileImpl) IsLocal() bool {
	return f.FileSystem.IsLocal()
}
func (f *FileImpl) Remove() error {
	return f.FileSystem.Remove(f.Path)
}
func (f *FileImpl) MoveFrom(src FileSystem, id string) (uint64, error) {
	logger.LogVerbose("Moving file ", path.Join(src.GetPath(), f.Path), " to ", f.FileSystem.GetPath())
	size, err := f.FileSystem.DownloadFile(src, f.Path, id)
	f.CachedSize = size
	if err != nil {
		return 0, err
	}
	if err := src.Remove(f.Path); err != nil {
		return 0, err
	}
	return size, nil
}
func (f *FileImpl) MoveTo(dest FileSystem, id string) (File, error) {
	destFile := &FileImpl{
		FileSystem: dest,
		Path:       f.Path,
	}
	size, err := destFile.MoveFrom(f.FileSystem, id)
	destFile.CachedSize = size
	return destFile, err
}
