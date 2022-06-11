package filesystem

import (
	"errors"
	"fmt"
	"github.com/hirochachacha/go-smb2"
	"io"
	"log"
	"net"
	"path"
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
	log.Println("cleaning ", base)
	share, err, cleanup := f.smbMount(base)
	defer cleanup()
	if err != nil {
		return nil, 0, err
	}
	fs := share.DirFS(".")
	return cleanFiles(share, fs, lookup)
}

func (f *SmbFileSystem) DownloadFile(fs FileSystem, filename string) (uint64, error) {
	dir, filename, ok := strings.Cut(strings.TrimPrefix(filename, "/"), "/")
	if !ok {
		return 0, errors.New("invalid path")
	}
	share, err, cleanup := f.smbMount(dir)
	defer cleanup()
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

	return copyFile(fs.GetFile(filename), file, path.Join(f.GetPath(), dir, filename))
}

func (f *SmbFileSystem) GetFile(filename string) File {
	return &FileImpl{Path: strings.TrimPrefix(filename, "/"), FileSystem: f}
}

func (f *SmbFileSystem) ReadFile(filename string) (io.Reader, func(), error) {
	path, filename, ok := strings.Cut(strings.TrimPrefix(filename, "/"), "/")
	if !ok {
		return nil, func() {}, errors.New("invalid path")
	}
	share, err, cleanup := f.smbMount(path)
	if err != nil {
		return nil, cleanup, err
	}
	file, err := share.Open(filename)
	return file, func() { _ = file.Close(); cleanup() }, err
}

func (f *SmbFileSystem) GetPath() string {
	return "//" + f.Host + "/"
}

func (f *SmbFileSystem) GetSize(filename string) uint64 {
	path, filename, ok := strings.Cut(strings.TrimPrefix(filename, "/"), "/")
	if !ok {
		return 0
	}
	share, err, cleanup := f.smbMount(path)
	defer cleanup()
	if err != nil {
		return 0
	}

	file, err := share.Open(filename)
	if err != nil {
		return 0
	}
	stat, err := file.Stat()
	return uint64(stat.Size())
}

func (f *SmbFileSystem) IsLocal() bool {
	return false
}
func (f *SmbFileSystem) Mkdir(dir string) error {
	path, dir, ok := strings.Cut(strings.TrimPrefix(dir, "/"), "/")
	if !ok {
		return errors.New("invalid path")
	}
	share, err, cleanup := f.smbMount(path)
	defer cleanup()
	if err != nil {
		return err
	}

	return share.MkdirAll(dir, 0755)
}

func (f *SmbFileSystem) RemoveAll(dir string) error {
	path, dir, ok := strings.Cut(strings.TrimPrefix(dir, "/"), "/")
	if !ok {
		return errors.New("invalid path")
	}
	share, err, cleanup := f.smbMount(path)
	defer cleanup()
	if err != nil {
		return err
	}

	return share.RemoveAll(dir)
}

func (f *SmbFileSystem) Remove(filename string) error {
	path, filename, ok := strings.Cut(strings.TrimPrefix(filename, "/"), "/")
	if !ok {
		return errors.New("invalid path")
	}
	share, err, cleanup := f.smbMount(path)
	defer cleanup()
	if err != nil {
		return err
	}

	return share.Remove(strings.TrimPrefix(filename, "/"))
}

func (f *SmbFileSystem) smbMount(path string) (*smb2.Share, error, func()) {
	addr, _, _ := strings.Cut(f.Host, ":")
	fmt.Printf(" Mounting //%s/%s", addr, path)

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
	fmt.Printf("\r")
	return s, err, cleanup
}

func (f *SmbFileSystem) splitPath(path string) (string, string) {
	path = strings.TrimPrefix(path, "/")
	base, dir, ok := strings.Cut(path, "/")
	if !ok {
		return base, ""
	}
	return base, dir
}
