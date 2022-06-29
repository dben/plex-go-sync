package filesystem

import (
	"errors"
	"github.com/dustin/go-humanize"
	"github.com/hirochachacha/go-smb2"
	"io"
	"net"
	"path"
	"plex-go-sync/internal/logger"
	"strings"
	"sync"
)

type SmbFileSystem struct {
	Host     string
	Path     string
	Username string
	Password string
}

var mutex = &sync.Mutex{}

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

func (f *SmbFileSystem) GetFreeSpace(base string) (uint64, error) {
	share, _, err := f.smbMount(base)
	if err != nil {
		return 0, err
	}
	stat, err := share.Statfs(".")
	if err != nil {
		return 0, err
	}
	logger.LogVerbose("Free space: ", stat.FreeBlockCount()*stat.BlockSize())
	return stat.FreeBlockCount() * stat.BlockSize(), nil
}

func (f *SmbFileSystem) Clean(base string, lookup map[string]bool) (map[string]uint64, int64, error) {
	logger.LogInfo("Cleaning ", base)
	share, _, err := f.smbMount(base)
	if err != nil {
		return nil, 0, err
	}
	fs := share.DirFS(".")
	return cleanFiles(share, fs, lookup)
}

func (f *SmbFileSystem) DownloadFile(fs FileSystem, filepath string, id string) (uint64, error) {
	share, filename, err := f.smbMount(filepath)
	if err != nil {
		return 0, err
	}
	if filename == "" {
		return 0, errors.New("invalid path")
	}
	if err = share.MkdirAll(path.Dir(filename), 0755); err != nil {
		return 0, err
	}

	if stat, err := share.Stat(filename); err == nil {
		return uint64(stat.Size()), nil
	}

	file, err := share.Create(filename)
	if err != nil {
		return 0, err
	}

	return copyFile(fs.GetFile(filepath), file, path.Join(f.GetPath(), filepath), id)
}

func (f *SmbFileSystem) FileWriter(filepath string) (io.WriteCloser, error) {
	share, filename, err := f.smbMount(filepath)
	if err != nil {
		return nil, err
	}
	if filename == "" {
		return nil, errors.New("invalid path")
	}

	logger.LogVerbose("Creating directory ", path.Join(f.GetPath()+path.Dir(filepath)))
	err = share.MkdirAll(path.Dir(filename), 0755)
	if err != nil {
		return nil, err
	}
	logger.LogVerbose(f.GetPath() + filepath)

	return share.Create(filename)
}

func (f *SmbFileSystem) GetFile(filepath string) File {
	return &FileImpl{Path: strings.TrimPrefix(filepath, "/"), FileSystem: f}
}

func (f *SmbFileSystem) ReadFile(filepath string) (io.ReadCloser, error) {
	share, filename, err := f.smbMount(filepath)
	if err != nil {
		return nil, err
	}
	if filename == "" {
		return nil, errors.New("invalid path")
	}
	logger.LogVerbosef("Reading file (%s)\n", filename)
	return share.Open(filename)
}

func (f *SmbFileSystem) GetPath() string {
	return "//" + f.Host + "/"
}

func (f *SmbFileSystem) GetSize(filepath string) uint64 {
	share, filename, err := f.smbMount(filepath)
	if err != nil {
		return 0
	}
	if filename == "" {
		return 0
	}

	file, err := share.Open(filename)
	if err != nil {
		return 0
	}
	stat, err := file.Stat()
	logger.LogVerbosef("Size of file (%s%s): %s\n", f.Path, filename, humanize.Bytes(uint64(stat.Size())))

	return uint64(stat.Size())
}

func (f *SmbFileSystem) IsLocal() bool {
	return false
}
func (f *SmbFileSystem) Mkdir(dir string) error {
	share, filename, err := f.smbMount(dir)
	if err != nil {
		return err
	}
	if filename == "" {
		return errors.New("invalid path")
	}

	logger.LogVerbose("Creating directory ", path.Join(f.GetPath(), dir))

	return share.MkdirAll(dir, 0755)
}

func (f *SmbFileSystem) RemoveAll(dir string) error {
	share, filename, err := f.smbMount(dir)
	if err != nil {
		return err
	}
	if filename == "" {
		return errors.New("invalid path")
	}

	logger.LogVerbose("Removing directory ", path.Join(f.GetPath(), dir))
	return share.RemoveAll(dir)
}

func (f *SmbFileSystem) Remove(filepath string) error {
	share, filename, err := f.smbMount(filepath)
	if err != nil {
		return err
	}
	if filename == "" {
		return errors.New("invalid path")
	}

	logger.LogVerbose("Removing file ", path.Join(f.GetPath(), filepath))
	return share.Remove(filename)
}

type smbConnection struct {
	conn    net.Conn
	dialer  *smb2.Dialer
	session *smb2.Session
	shares  map[string]*smb2.Share
}

var smbConnections = make(map[string]*smbConnection)

func NewSmbConnection(host string, username string, password string) (*smbConnection, error) {
	c := &smbConnection{}
	var err error
	c.conn, err = net.Dial("tcp", host)
	if err != nil {
		_ = c.conn.Close()
		return nil, err
	}

	c.dialer = &smb2.Dialer{
		Initiator: &smb2.NTLMInitiator{
			User:     username,
			Password: password,
		},
	}

	c.session, err = c.dialer.Dial(c.conn)
	if err != nil {
		_ = c.session.Logoff()
		_ = c.conn.Close()
		return nil, err
	}

	c.shares = make(map[string]*smb2.Share)
	return c, nil
}

func CloseAllSmbConnections() {
	for _, c := range smbConnections {
		_ = c.session.Logoff()
		_ = c.conn.Close()
	}
}

func (f *SmbFileSystem) smbMount(filepath string) (*smb2.Share, string, error) {
	mutex.Lock()
	defer mutex.Unlock()
	base, filename, _ := strings.Cut(strings.TrimPrefix(filepath, "/"), "/")
	addr, _, _ := strings.Cut(f.Host, ":")
	var err error
	var conn = smbConnections[addr]
	if conn != nil {
		_, err = conn.session.ListSharenames()
	}
	if err != nil {
		_ = conn.session.Logoff()
		_ = conn.conn.Close()
		smbConnections[addr] = nil
	}
	if smbConnections[addr] == nil || err != nil {
		logger.LogInfo("Mounting //", addr, "/", base)
		conn, err = NewSmbConnection(f.Host, f.Username, f.Password)
		if err != nil {
			return nil, filename, err
		}
		smbConnections[addr] = conn
	}

	if conn.shares[base] != nil {
		_, err = conn.shares[base].Stat(".")
	}
	if err != nil {
		_ = conn.shares[base].Umount()
	}
	if conn.shares[base] == nil || err != nil {
		conn.shares[base], err = conn.session.Mount("//" + addr + "/" + base)
		if err != nil {
			if conn.shares[base] != nil {
				_ = conn.shares[base].Umount()
			}
			return nil, filename, err
		}
	}
	return conn.shares[base], filename, nil
}
