package filesystem

import (
	"io"
	"log"
	"os"
	"path"
	"strings"
)

type LocalFileSystem struct {
	Path string
}

func NewLocalFileSystem(dir string) FileSystem {
	return &LocalFileSystem{Path: dir}
}
func (f *LocalFileSystem) Clean(base string, lookup map[string]bool) (map[string]uint64, uint64, error) {
	log.Println("Cleaning ", base)
	dir := path.Join(f.Path, strings.TrimPrefix(base, "/"))
	return cleanFiles(&osImpl{}, os.DirFS(dir), lookup)
}

func (f *LocalFileSystem) DownloadFile(fs FileSystem, filename string) (uint64, error) {
	absPath := path.Join(f.Path, strings.TrimPrefix(filename, "/"))

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

	return copyFile(fs.GetFile(strings.TrimPrefix(filename, "/")), file, absPath)

}

func (f *LocalFileSystem) GetFile(filename string) File {
	return &FileImpl{Path: strings.TrimPrefix(filename, "/"), FileSystem: f}
}

func (f *LocalFileSystem) ReadFile(filename string) (io.Reader, func(), error) {
	file, err := os.Open(path.Join(f.Path, strings.TrimPrefix(filename, "/")))
	return file, func() { _ = file.Close() }, err
}

func (f *LocalFileSystem) GetPath() string {
	return f.Path
}

func (f *LocalFileSystem) GetSize(filename string) uint64 {
	stat, err := os.Stat(path.Join(f.Path, strings.TrimPrefix(filename, "/")))
	if err != nil {
		return 0
	}

	return uint64(stat.Size())
}

func (f *LocalFileSystem) IsLocal() bool {
	return true
}

func (f *LocalFileSystem) Mkdir(dir string) error {
	return os.MkdirAll(path.Join(f.Path, strings.TrimPrefix(dir, "/")), 0755)
}

func (f *LocalFileSystem) RemoveAll(dir string) error {
	return os.RemoveAll(path.Join(f.Path, strings.TrimPrefix(dir, "/")))
}

func (f *LocalFileSystem) Remove(dir string) error {
	return os.Remove(path.Join(f.Path, strings.TrimPrefix(dir, "/")))
}
