package filesystem

import (
	"io"
	"path"
)

type File interface {
	CopyFrom(fs FileSystem) (uint64, error)
	CopyTo(fs FileSystem) (File, error)
	MoveFrom(fs FileSystem) (uint64, error)
	MoveTo(fs FileSystem) (File, error)
	ReadFile() (io.Reader, func(), error)
	GetRelativePath() string
	GetAbsolutePath() string
	GetSize() uint64
	IsLocal() bool
	Remove() error
}
type FileImpl struct {
	FileSystem FileSystem
	Path       string
	CachedSize uint64
}

func NewFile(base string, file string) File {
	return &FileImpl{
		FileSystem: NewFileSystem(base),
		Path:       file,
	}
}
func (f *FileImpl) CopyFrom(src FileSystem) (uint64, error) {
	size, err := f.FileSystem.DownloadFile(src, f.Path)
	f.CachedSize = size
	return size, err
}
func (f *FileImpl) CopyTo(dest FileSystem) (File, error) {
	size, err := dest.DownloadFile(f.FileSystem, f.Path)
	destFile := &FileImpl{
		FileSystem: dest,
		Path:       f.Path,
		CachedSize: size,
	}
	return destFile, err
}
func (f *FileImpl) ReadFile() (io.Reader, func(), error) {
	return f.FileSystem.ReadFile(f.Path)
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
func (f *FileImpl) MoveFrom(src FileSystem) (uint64, error) {
	size, err := f.FileSystem.DownloadFile(src, f.Path)
	f.CachedSize = size
	if err != nil {
		return 0, err
	}
	if err := src.Remove(f.Path); err != nil {
		return 0, err
	}
	return size, nil
}
func (f *FileImpl) MoveTo(dest FileSystem) (File, error) {
	destFile := &FileImpl{
		FileSystem: dest,
		Path:       f.Path,
	}
	size, err := destFile.MoveFrom(f.FileSystem)
	destFile.CachedSize = size
	if err != nil {
		return destFile, err
	}
	return destFile, nil
}
