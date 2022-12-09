package test

import (
	"context"
	"github.com/dustin/go-humanize"
	"io"
	"io/fs"
	"os"
	"plex-go-sync/internal/filesystem"
	"strings"
)

type discardCloser struct {
	io.Writer
}

func (discardCloser) Close() error {
	return nil
}

type TestFileSystem struct {
	Path string
}

func NewTestFileSystem(dir string) filesystem.FileSystem {
	return TestFileSystem{Path: dir}
}

func (f *TestFileSystem) GetFreeSpace(base string) (uint64, error) {
	return humanize.GByte * 100, nil
}

func (f *TestFileSystem) GetFileSystem(base string) (fs.FS, error) {
	return os.DirFS("/tmp"), nil
}

func (f *TestFileSystem) IsEmptyDir(dir string) bool {
	return false
}

func (f *TestFileSystem) DownloadFile(ctx *context.Context, fs filesystem.FileSystem, filename string, id string) (uint64, error) {
	return humanize.MByte * 100, nil
}

func (f *TestFileSystem) GetFile(filename string) filesystem.File {
	return &filesystem.FileImpl{Path: strings.TrimPrefix(filename, "/"), FileSystem: f}
}

func (f *TestFileSystem) ReadFile(filename string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("test")), nil
}

func (f *TestFileSystem) FileWriter(filename string) (io.WriteCloser, error) {
	return discardCloser{Writer: io.Discard}, nil
}

func (f *TestFileSystem) GetPath() string {
	return f.Path
}

func (f *TestFileSystem) GetSize(filename string) (uint64, error) {
	return humanize.GByte * 100, nil
}

func (f *TestFileSystem) IsLocal() bool {
	return true
}

func (f *TestFileSystem) Mkdir(dir string) error {
	return nil
}

func (f *TestFileSystem) RemoveAll(dir string) error {
	return nil
}

func (f *TestFileSystem) Remove(dir string) error {
	return nil
}
