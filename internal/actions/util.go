package actions

import (
	"encoding/json"
	"errors"
	"fmt"
	ffmpeg_go "github.com/u2takey/ffmpeg-go"
	"github.com/urfave/cli"
	"log"
	"math"
	"math/rand"
	"os"
	"path"
	. "plex-go-sync/internal/filesystem"
	"plex-go-sync/internal/plex"
	"strconv"
	"strings"
	"time"
)

func ReadConfig(configFile string, args *cli.Context) (*Config, error) {
	var config Config
	log.Println("Loading config...")
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

	writer := json.NewEncoder(os.Stdout)
	writer.SetIndent("", "  ")
	_ = writer.Encode(config)

	return &config, err
}

func GetPlaylistItems(name string, server string, token string) ([]PlaylistItem, map[string]bool, error) {
	var results []PlaylistItem
	plexServer, err := plex.New(server, token)
	if err != nil {
		log.Println("Failed to connect to plex: ", err)
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
	return results, hashmap, err
}

func reencodeVideo(src FileSystem, dest FileSystem, file string, config *Config) (File, error) {
	slice := strings.Split(file, ".")
	outFile := strings.Join(slice[:len(slice)-1], ".") + ".mp4"

	destFile, err := getLocalFile(dest.GetFile(outFile), path.Join(config.TempDir, "dest"))
	if err != nil {
		return destFile, err
	}
	srcFile, isSrcCopy, err := getOrCopyLocalFile(src.GetFile(file), path.Join(config.TempDir, "src"))
	if err != nil {
		return destFile, err
	}

	progress, msg := FfmpegConverter(srcFile, destFile, ffmpeg_go.KwArgs{
		"crf": 23, "s": "1280x720", "format": "mp4", "loglevel": "warning",
	})

	completed := false
	for {
		if completed {
			break
		}
		select {
		case data, more := <-progress:
			completed = !more
			if more {
				percent := int(math.Round(100 * data.OutTime / data.Duration))
				if percent < 1 {
					percent = 1
				}
				bar := strings.Repeat("#", percent/2)
				remaining := time.Duration((float64(data.Elapsed) / data.OutTime) * (data.Duration - data.OutTime))
				fmt.Printf("\r[%-50s]%3d%% at %sx, %s remaining         ", bar, percent, data.Speed, remaining.Round(time.Second).String())
				//_ = json.NewEncoder(os.Stdout).Encode(data)
			}
		case err = <-msg:
			log.Println(err)
			return destFile, err
		}
	}
	fmt.Println()

	if isSrcCopy {
		log.Println("Removing src")
		if err := srcFile.Remove(); err != nil {
			log.Println("error removing temp file: ", err)
		}
	}
	return destFile, nil

}

func ifItFitsItSits(tmpFile File, dest FileSystem, remainingBytes uint64) (bool, error) {
	var err error
	if !dest.IsLocal() && tmpFile.GetSize() <= remainingBytes {
		_, err = tmpFile.MoveTo(dest)
	} else if dest.IsLocal() && tmpFile.GetSize() > remainingBytes {
		if err = tmpFile.Remove(); err != nil {
			log.Println("error removing dest file", err)
		}

	}
	if tmpFile.GetSize() > remainingBytes {
		return false, err
	}
	return true, nil
}

func removeLast(bytes uint64, dest FileSystem, items []PlaylistItem, existing map[string]uint64) uint64 {
	removed := uint64(0)
	for i := len(items) - 1; i >= 0; i-- {
		key := items[i].Path
		if existing[key] != 0 {
			if err := dest.Remove(key); err != nil {
				log.Println("error removing " + key + ": " + err.Error())
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

func getOrCopyLocalFile(f File, tempDir string) (File, bool, error) {
	if f.IsLocal() {
		return f, false, nil
	}
	log.Println("Copying to temp directory...")
	tmpFolder := NewLocalFileSystem(tempDir)
	err := tmpFolder.Mkdir(path.Dir(f.GetRelativePath()))
	if err == nil {
		_, err = f.CopyTo(tmpFolder)
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
