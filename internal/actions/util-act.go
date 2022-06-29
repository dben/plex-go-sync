package actions

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dustin/go-humanize"
	ffmpeg_go "github.com/u2takey/ffmpeg-go"
	"github.com/urfave/cli/v2"
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

	parents := make(map[string]bool)
	hashmap := make(map[string]bool)

	for _, item := range items.MediaContainer.Metadata {

		// find 720p if exists
		bestMedia := item.Media[0]
		for _, media := range item.Media {
			if len(media.Part) != 1 {
				continue
			}
			if media.Height <= heightFilter && (bestMedia.Height > heightFilter || media.Height > bestMedia.Height) {
				bestMedia = media
			}
		}

		if len(bestMedia.Part) != 1 {
			// multipart files - not supported yet
			continue
		}

		newItem := PlaylistItem{Path: item.Media[0].Part[0].File, Parent: item.GrandparentTitle}

		results = append(results, newItem)
		parents[item.GrandparentTitle] = true
		hashName := strings.TrimPrefix(newItem.Path, "/")
		ext := path.Ext(hashName)
		hashName = strings.TrimSuffix(hashName, ext)
		hashmap[hashName] = true
	}

	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(results), func(i, j int) {
		results[i], results[j] = results[j], results[i]
	})
	if items.MediaContainer.Metadata[0].Type != "episode" {
		logger.LogVerbose("Playlist ", name, " retrieved, ", len(results), " items")
		return results, hashmap, err
	}
	var i = 0
	for i < len(results) {
		for key, _ := range parents {
			if results[i].Parent == key {
				i++
				continue
			}
			for j := i + 1; j < len(results); j++ {
				if results[j].Parent == key {
					results[i], results[j] = results[j], results[i]
					break
				}
			}
			if results[i].Parent == key {
				i++
				continue
			}
			logger.LogVerbose("End of ", key, " at ", i)
			delete(parents, key)
		}

	}

	logger.LogVerbose("Playlist ", name, " retrieved, ", len(results), " items")
	return results, hashmap, err
}

func reencodeVideo(id string, src FileSystem, dest FileSystem, file string, tempDir string, fast bool) (File, uint64, error) {
	slice := strings.Split(file, ".")
	outFile := strings.Join(slice[:len(slice)-1], ".") + ".mp4"
	var totalSize uint64

	reader, err := src.GetFile(file).ReadFile()
	if err != nil {
		logger.LogError("error reading file: ", err)
		return nil, 0, err
	}
	copyFile, duration, audioStreams, _ := Ffprobe(reader)
	srcFile := src.GetFile(file)
	destFile := dest.GetFile(outFile)
	// Just copy the file
	if copyFile && strings.HasSuffix(file, MediaFormat) {
		logger.LogVerbose("Copying file: ", file)
		size, err := destFile.CopyFrom(src, id+outFile)
		return destFile, size, err
	}
	var kwargs ffmpeg_go.KwArgs
	if copyFile {
		kwargs = ffmpeg_go.KwArgs{
			"vcodec":   "copy",
			"movflags": "frag_keyframe+empty_moov",
			"format":   MediaFormat,
			"loglevel": "error", "y": "",
		}
	} else if fast {
		return nil, 0, errors.New("file must be converted, skipping because we are in -fast mode")
	} else {
		kwargs = ffmpeg_go.KwArgs{
			"c:v": "libx264",
			//"tag:v":    "hvc1",
			"movflags": "frag_keyframe+empty_moov",
			"crf":      strconv.Itoa(crfFilter),
			"s":        fmt.Sprintf("%dx%d", widthFilter, heightFilter),
			"format":   MediaFormat,
			"loglevel": "error", "y": "",
		}
	}

	if len(audioStreams) == 0 {
		kwargs["c:a"] = "aac"
	} else {
		var maps = []string{"0:v"}
		for i := range audioStreams {
			maps = append(maps, "0:a:"+strconv.Itoa(i))
		}
		kwargs["map"] = maps
	}

	totalSize, err = tryConvert(srcFile, destFile, duration, kwargs, tempDir, id+outFile)
	if err != nil {
		return nil, 0, err
	}

	if slice[len(slice)-1] == "mp4" && totalSize > 0 {
		srcSize := srcFile.GetSize()
		if totalSize >= srcSize-(20*humanize.MiByte) {
			logger.LogInfo("Existing file smaller, using that")
			totalSize, err = destFile.CopyFrom(src, id+outFile)
			if err != nil {
				return nil, 0, err
			}
		}
	}
	if totalSize == 0 {
		err = errors.New("file has 0 bytes")
		logger.LogWarning(err)
		return destFile, 0, err
	}

	return destFile, totalSize, nil

}

func removeLast(bytes int64, dest FileSystem, items []PlaylistItem, existing map[string]uint64) int64 {
	removed := int64(0)
	for i := len(items) - 1; i >= 0; i-- {
		_, key, _ := strings.Cut(strings.TrimPrefix(items[i].Path, "/"), "/")
		if existing[key] != 0 {
			logger.LogVerbose("Removing ", items[i].Path)
			if err := dest.Remove(items[i].Path); err != nil {
				logger.LogWarning("error removing ", items[i].Path, ": ", err.Error())
			} else {
				bytes -= int64(existing[key])
				removed += int64(existing[key])
				delete(existing, key)
			}
		}
		if bytes <= 0 {
			break
		}
	}
	return removed
}

//goland:noinspection GoUnhandledErrorResult
func tryConvert(in File, out File, duration time.Duration, args ffmpeg_go.KwArgs, tempDir string, id string) (uint64, error) {
	logger.LogVerbose("Try ffmpeg convert: ", out.GetRelativePath())
	progress, msg := FfmpegConverter(in, out, duration, args)
	totalSize, err := watchProgress(progress, msg, id)
	if _, ok := err.(*InputBufferError); ok && !in.IsLocal() {
		logger.LogInfo("Copying to temp directory...")
		tmpFolder := NewLocalFileSystem(path.Join(tempDir, "src"))
		if err := tmpFolder.Mkdir(path.Dir(in.GetRelativePath())); err != nil {
			return 0, err
		}
		if _, err := in.CopyTo(tmpFolder, id); err != nil {
			return 0, err
		}
		tmpFile := tmpFolder.GetFile(in.GetRelativePath())
		defer tmpFile.Remove()
		return tryConvert(tmpFile, out, duration, args, tempDir, id)
	} else if _, ok := err.(*OutputBufferError); ok && !out.IsLocal() {
		logger.LogInfo("Making temp directory...")
		tmpFolder := NewLocalFileSystem(path.Join(tempDir, "dest"))
		if err := tmpFolder.Mkdir(path.Dir(out.GetRelativePath())); err != nil {
			return 0, err
		}
		tmpFile := tmpFolder.GetFile(out.GetRelativePath())
		defer tmpFile.Remove()
		totalSize, err = tryConvert(tmpFile, out, duration, args, tempDir, id)
		if err != nil {
			return totalSize, err
		}
		return out.CopyFrom(tmpFile.GetFileSystem(), id)
	} else {
		return totalSize, err
	}
}

func watchProgress(progress chan FfmpegProps, errChan chan error, id string) (uint64, error) {
	totalSize := uint64(0)
	for {
		select {
		case data, more := <-progress:
			if more {
				percent := float64(data.OutTime) / float64(data.Duration)
				remaining := time.Duration((float64(data.Elapsed) / float64(data.OutTime+time.Second)) * float64(data.Duration-data.OutTime))
				logger.Progress(id, percent, " at ", data.Speed+"x ", remaining.Round(time.Second).String(), " remaining")

				if totalSize < data.TotalSize {
					totalSize = data.TotalSize
				}
			} else {
				logger.ProgressClear(id)
			}
		case err, more := <-errChan:
			if err != nil {
				logger.ProgressClear(id)
				logger.LogWarning(err)
				return totalSize, err
			}

			if !more {
				logger.ProgressClear(id)
				logger.LogInfo("Encoding completed: ", humanize.Bytes(totalSize))
				return totalSize, err
			}
		}
	}
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

func humanizeNegBytes(val int64) string {
	if val < 0 {
		return "-" + humanize.Bytes(uint64(val*-1))
	}
	return humanize.Bytes(uint64(val))
}
