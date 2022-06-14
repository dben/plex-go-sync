package actions

import (
	"encoding/json"
	"errors"
	"github.com/dustin/go-humanize"
	"github.com/urfave/cli"
	"math/rand"
	"os"
	"path"
	. "plex-go-sync/internal/filesystem"
	"plex-go-sync/internal/logger"
	"plex-go-sync/internal/plex"
	"strconv"
	"strings"
	"time"
)

const KILO = 1000

func ReadConfig(configFile string, args *cli.Context) (*Config, error) {
	var config Config
	logger.LogInfo("Loading config...")
	file, err := os.Open(configFile)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return nil, err
	}

	if args.String("server") != "" {
		config.Server = args.String("server")
	}
	if args.String("token") != "" {
		config.Token = args.String("token")
	}
	if args.String("destination-server") != "" {
		config.DestinationServer = args.String("destination-server")
	}
	if args.StringSlice("playlist") != nil && len(args.StringSlice("playlist")) > 0 {
		for i := 0; i < len(args.StringSlice("playlist")); i++ {
			config.Playlists = append(config.Playlists, *NewPlaylist(args.StringSlice("playlist")[i], args.StringSlice("size")[i]))
		}
	}

	if logger.LogLevel == "VERBOSE" {
		writer := json.NewEncoder(os.Stdout)
		writer.SetIndent("", "  ")
		_ = writer.Encode(config)
	}

	return &config, err
}

func GetPlaylistItems(name string, server string, token string) ([]PlaylistItem, map[string]bool, error) {
	var results []PlaylistItem
	plexServer, err := plex.New(server, token)
	if err != nil {
		logger.LogError("Failed to connect to plex: ", err)
		return nil, nil, err
	}

	playlist, err := plexServer.GetPlaylistsByName(name)
	if err != nil {
		return nil, nil, err
	}
	if len(playlist.MediaContainer.Metadata) == 0 {
		return nil, nil, errors.New("no playlist found")
	}
	key, err := strconv.Atoi(playlist.MediaContainer.Metadata[0].RatingKey)
	if err != nil {
		return nil, nil, err
	}
	items, err := plexServer.GetPlaylist(key)
	if err != nil {
		return nil, nil, err
	}

	for _, item := range items.MediaContainer.Metadata {

		// find 720p if exists
		bestMedia := item.Media[0]
		for _, media := range item.Media {
			if media.Width >= 720 && (bestMedia.Width < 720 || media.Width < bestMedia.Width) {
				bestMedia = media
			}
		}

		if len(bestMedia.Part) != 1 {
			// multipart files - not supported yet
			continue
		}

		results = append(results, PlaylistItem{Path: item.Media[0].Part[0].File})
	}

	hashmap := make(map[string]bool)
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(results), func(i, j int) {
		results[i], results[j] = results[j], results[i]
		hashName := strings.TrimPrefix(results[i].Path, "/")
		_, dir, ok := strings.Cut(hashName, "/")
		if !ok {
			return
		}
		split := strings.Split(dir, ".")
		if len(split) == 1 { // TODO: throw an error here
			return
		}
		hashmap[strings.Join(split[:len(split)-1], ".")] = true
	})
	logger.LogVerbose("Playlist ", name, " retrieved, ", len(results), " items")
	return results, hashmap, err
}

func reencodeVideo(id string, src FileSystem, dest FileSystem, file string, config *Config) (File, uint64, error) {
	slice := strings.Split(file, ".")
	outFile := strings.Join(slice[:len(slice)-1], ".") + ".mp4"

	destFile, err := getLocalFile(dest.GetFile(outFile), path.Join(config.TempDir, "dest"))
	if err != nil {
		logger.LogWarning("error getting dest file: ", err)
		return destFile, 0, err
	}
	srcFile, isSrcCopy, err := getOrCopyLocalFile(id, src.GetFile(file), path.Join(config.TempDir, "src"))
	if err != nil {
		logger.LogWarning("error getting src file: ", err)
		return destFile, 0, err
	}

	progress, msg := FfmpegConverter(srcFile, destFile, 3500*KILO, 720)
	totalSize := uint64(0)
	completed := false
	for {
		if completed {
			break
		}
		select {
		case data, more := <-progress:
			completed = !more
			if more {
				percent := float64(data.OutTime) / float64(data.Duration)
				if percent < .01 {
					percent = .01
				}
				remaining := time.Duration((float64(data.Elapsed) / float64(data.OutTime+time.Second)) * float64(data.Duration-data.OutTime))
				logger.Progress(id+outFile, percent, " at ", data.Speed+"x ", remaining.Round(time.Second).String(), " remaining")

				if totalSize < data.TotalSize {
					totalSize = data.TotalSize
				}
			} else {
				logger.ProgressClear(id + outFile)
				logger.LogInfo("Encoding completed: ", humanize.Bytes(totalSize))
			}
		case err = <-msg:
			logger.LogWarning(err)
			if err != nil {
				return destFile, totalSize, err
			}
		}
	}

	srcSize := srcFile.GetSize()
	if totalSize > 0 && totalSize >= srcSize-(20*humanize.MiByte) {
		return srcFile, srcSize, nil
	}

	if isSrcCopy {
		go func() {
			time.Sleep(time.Minute)
			logger.LogVerbose("Removing src", srcFile.GetAbsolutePath())
			if err := srcFile.Remove(); err != nil {
				logger.LogWarning("error removing temp file: ", err)
			}
		}()
	}
	if totalSize == 0 {
		err = errors.New("file has 0 bytes")
		logger.LogWarning(err)
		return destFile, 0, err
	}

	return destFile, totalSize, nil

}

func ifItFitsItSits(id string, tmpFile File, dest FileSystem, remainingBytes uint64) (bool, error) {
	var err error
	if !dest.IsLocal() && tmpFile.GetSize() <= remainingBytes {
		_, err = tmpFile.MoveTo(dest, id)
	} else if dest.IsLocal() && tmpFile.GetSize() > remainingBytes {
		if err = tmpFile.Remove(); err != nil {
			logger.LogWarning("error removing dest file: ", err)
		}
	} else if tmpFile.GetSize() > remainingBytes {
		return false, err
	}
	return true, err
}

func removeLast(bytes uint64, dest FileSystem, items []PlaylistItem, existing map[string]uint64) uint64 {
	removed := uint64(0)
	for i := len(items) - 1; i >= 0; i-- {
		key := items[i].Path
		if existing[key] != 0 {
			if err := dest.Remove(key); err != nil {
				logger.LogWarning("error removing ", key, ": ", err.Error())
			} else {
				bytes -= existing[key]
				removed += existing[key]
				delete(existing, key)
			}
		}
		if bytes <= 0 {
			break
		}
	}
	return removed
}

func getLocalFile(f File, tempDir string) (File, error) {
	if f.IsLocal() {
		return f, nil
	}
	tmpFolder := NewLocalFileSystem(tempDir)
	err := tmpFolder.Mkdir(path.Dir(f.GetRelativePath()))
	return tmpFolder.GetFile(f.GetRelativePath()), err

}

func getOrCopyLocalFile(id string, f File, tempDir string) (File, bool, error) {
	if f.IsLocal() {
		return f, false, nil
	}
	logger.LogInfo("Copying to temp directory...")
	tmpFolder := NewLocalFileSystem(tempDir)
	err := tmpFolder.Mkdir(path.Dir(f.GetRelativePath()))
	if err == nil {
		_, err = f.CopyTo(tmpFolder, id)
	}
	return tmpFolder.GetFile(f.GetRelativePath()), true, err
}
func getBase(items []PlaylistItem) (string, error) {
	if len(items) == 0 {
		return "", errors.New("no items")
	}
	base, _, ok := strings.Cut(strings.TrimPrefix(items[0].Path, "/"), "/")
	if !ok {
		return "", errors.New("invalid path structure")
	}
	return base, nil
}
