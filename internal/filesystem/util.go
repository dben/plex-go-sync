package filesystem

import (
	"fmt"
	"github.com/dustin/go-humanize"
	"io"
	iofs "io/fs"
	"log"
	"strings"
)

func cleanFiles(remove removeFS, fs iofs.FS, lookup map[string]bool) (map[string]uint64, uint64, error) {
	results := make(map[string]uint64)
	totalSize := uint64(0)
	err := iofs.WalkDir(fs, ".", func(path string, info iofs.DirEntry, err error) error {
		if info.IsDir() {
			return nil
		}
		key := path
		split := strings.Split(path, ".")
		if len(split) > 1 {
			key = strings.Join(split[:len(split)-1], ".")
		}
		if lookup[key] == true {
			detail, err := info.Info()
			if err != nil {
				return err
			}
			results[path] = uint64(detail.Size())
			totalSize += uint64(detail.Size())
			return nil
		}

		if err = remove.Remove(path); err != nil {
			log.Println("Removing: ", path, err.Error())
		} else {
			log.Println("Removing: ", path)
		}
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	return results, totalSize, nil
}

//goland:noinspection GoUnhandledErrorResult
func copyFile(from File, to io.ReaderFrom, toPath string) (uint64, error) {
	pReader, pWriter := io.Pipe()
	size := from.GetSize()
	log.Printf("Copying %s \n", from.GetAbsolutePath())
	log.Printf("     to %s\n", toPath)

	reader, cleanup, err := from.ReadFile()
	defer pReader.Close()
	defer cleanup()

	if err != nil {
		return 0, err
	}

	go func() {
		buff := make([]byte, 1024*1024)
		currentSize := 0
		for {
			n, err := reader.Read(buff)
			if n != 0 {
				currentSize += n
				progress := float32(currentSize) / float32(size)
				bar := strings.Repeat("#", int(progress*50))
				fmt.Printf("\r[%-50s]%3d%% %s copied of %s     ", bar, int(progress*100), humanize.Bytes(uint64(currentSize)), humanize.Bytes(size))
				pWriter.Write(buff[:n])
			}
			if err != nil {
				pWriter.Close()
				fmt.Println()
				return
			}
		}
	}()

	written, err := to.ReadFrom(pReader)
	if err != nil {
		return uint64(written), err
	}

	log.Println()
	return uint64(written), nil
}
