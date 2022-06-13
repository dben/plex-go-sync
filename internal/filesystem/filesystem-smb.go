package filesystem

import (
	"errors"
	"fmt"
	"github.com/hirochachacha/go-smb2"
	"io"
	"net"
	"path"
	"plex-go-sync/internal/logger"
	"strings"
)

type SmbFileSystem struct {
	Host     string
	Path     string
	Username string
	Password string
}

func NewSmbFileSystem(dir string) FileSystem {
	if strings.HasPrefix(dir, "//") {
		dir = dir[2:]
	} else if strings.HasPrefix(dir, "smb://") {
		dir = dir[6:]
	} else {
		panic("invalid smb url")
	}

	username := ""
	password := ""
	credentials, dir, found := strings.Cut(dir, "@")
	if found {
		username, password, found = strings.Cut(credentials, ":")
	} else {
		dir = credentials
	}
	host, dir, _ := strings.Cut(dir, "/")

	if !strings.Contains(host, ":") {
		host = host + ":445"
	}
	return &SmbFileSystem{Path: dir, Host: host, Username: username, Password: password}
}
func (f *SmbFileSystem) Clean(base string, lookup map[string]bool) (map[string]uint64, uint64, error) {
	logger.LogInfo("Cleaning ", base)
	share, err, cleanup := f.smbMount(base)
	defer cleanup()
	if err != nil {
		return nil, 0, err
	}
	fs := share.DirFS(".")
	return cleanFiles(share, fs, lookup)
}

func (f *SmbFileSystem) DownloadFile(fs FileSystem, filename string) (uint64, error) {
	logger.LogVerbose("Copying file from ", fs.GetPath(), " to ", f.GetPath()+strings.TrimPrefix(filename, "/"))
	base, filename := f.splitPath(filename)
	if filename == "" {
		return 0, errors.New("invalid path")
	}
	share, err, cleanup := f.smbMount(base)
	defer cleanup()
	if err != nil {
		return 0, err
	}
	err = share.MkdirAll(path.Dir(filename), 0755)
	if err != nil {
		return 0, err
	}

	if stat, err := share.Stat(filename); err == nil {
		return uint64(stat.Size()), nil
	}

	file, err := share.Create(filename)
	if err != nil {
		return 0, err
	}

	return copyFile(fs.GetFile(path.Join(base, filename)), file, path.Join(f.GetPath(), base, filename))
}

func (f *SmbFileSystem) GetFile(filename string) File {
	return &FileImpl{Path: strings.TrimPrefix(filename, "/"), FileSystem: f}
}

func (f *SmbFileSystem) ReadFile(filename string) (io.Reader, func(), error) {
	base, filename := f.splitPath(filename)
	if filename == "" {
		return nil, func() {}, errors.New("invalid path")
	}
	share, err, cleanup := f.smbMount(base)
	if err != nil {
		return nil, cleanup, err
	}
	logger.LogVerbose("Reading file ", filename)
	file, err := share.Open(filename)
	return file, func() { _ = file.Close(); cleanup() }, err
}

func (f *SmbFileSystem) GetPath() string {
	return "//" + f.Host + "/"
}

func (f *SmbFileSystem) GetSize(filename string) uint64 {
	base, filename := f.splitPath(filename)
	if filename == "" {
		return 0
	}
	share, err, cleanup := f.smbMount(base)
	defer cleanup()
	if err != nil {
		return 0
	}

	file, err := share.Open(filename)
	if err != nil {
		return 0
	}
	stat, err := file.Stat()
	logger.LogVerbosef("Size of file (%s%s): %s\n", f.Path, filename, stat.Size())

	return uint64(stat.Size())
}

func (f *SmbFileSystem) IsLocal() bool {
	return false
}
func (f *SmbFileSystem) Mkdir(dir string) error {
	base, filename := f.splitPath(dir)
	if filename == "" {
		return errors.New("invalid path")
	}
	share, err, cleanup := f.smbMount(base)
	defer cleanup()
	if err != nil {
		return err
	}

	logger.LogVerbose("Creating directory ", path.Join(f.GetPath(), base, dir))

	return share.MkdirAll(dir, 0755)
}

func (f *SmbFileSystem) RemoveAll(dir string) error {
	base, filename := f.splitPath(dir)
	if filename == "" {
		return errors.New("invalid path")
	}
	share, err, cleanup := f.smbMount(base)
	defer cleanup()
	if err != nil {
		return err
	}

	logger.LogVerbose("Removing directory ", path.Join(f.GetPath(), base, dir))
	return share.RemoveAll(dir)
}

func (f *SmbFileSystem) Remove(filename string) error {
	base, filename := f.splitPath(filename)
	if filename == "" {
		return errors.New("invalid path")
	}
	share, err, cleanup := f.smbMount(base)
	defer cleanup()
	if err != nil {
		return err
	}

	logger.LogVerbose("Removing file ", path.Join(f.GetPath(), base, filename))
	return share.Remove(strings.TrimPrefix(filename, "/"))
}

func (f *SmbFileSystem) smbMount(path string) (*smb2.Share, error, func()) {
	addr, _, _ := strings.Cut(f.Host, ":")
	if logger.LogLevel != "WARN" && logger.LogLevel != "ERROR" {
		fmt.Printf("Mounting //%s/%s\r", addr, path)
	}
	if logger.LogLevel == "VERBOSE" {
		fmt.Println()
	}

	conn, err := net.Dial("tcp", f.Host)
	if err != nil {
		return nil, err, func() { _ = conn.Close() }
	}

	d := &smb2.Dialer{
		Initiator: &smb2.NTLMInitiator{
			User:     f.Username,
			Password: f.Password,
		},
	}

	c, err := d.Dial(conn)
	cleanup := func() {
		_ = c.Logoff()
		_ = conn.Close()
	}

	if err != nil {
		return nil, err, cleanup
	}
	s, err := c.Mount("//" + addr + "/" + path)
	return s, err, cleanup
}

func (f *SmbFileSystem) splitPath(path string) (string, string) {
	path = strings.TrimPrefix(path, "/")
	base, dir, ok := strings.Cut(path, "/")
	if !ok {
		logger.LogError("Invalid path: ", path)
		return base, ""
	}
	return base, dir
}
