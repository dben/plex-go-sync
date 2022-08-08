package actions

import (
	"errors"
	"fmt"
	"github.com/dustin/go-humanize"
	ffmpeg_go "github.com/u2takey/ffmpeg-go"
	"math"
	"path"
	. "plex-go-sync/internal/filesystem"
	"plex-go-sync/internal/logger"
	"plex-go-sync/internal/plex"
	"strconv"
	"strings"
	"sync"
	"time"
)

func CloneLibrary(playlist *Playlist, config *Config, src FileSystem, dest FileSystem, fast bool, wg *sync.WaitGroup, progress chan<- *Playlist) {
	existing, existingSize, base, err := CleanPlaylist(playlist, config, dest)
	playlistItems := playlist.Items

	if err != nil {
		logger.LogWarning("Skipping playlist: ", err.Error())
		wg.Done()
		return
	}

	logger.LogInfo("Cleaning ", path.Join(config.TempDir, "src", base), " directory")
	if config.TempDir != "" {
		if err = NewLocalFileSystem(config.TempDir).RemoveAll(path.Join("src", base)); err != nil {
			logger.LogWarning(err.Error())
		}
	}

	logger.LogInfo("Cleaning ", path.Join(config.TempDir, "dest", base), " directory")
	if config.TempDir != "" {
		if err = NewLocalFileSystem(config.TempDir).RemoveAll(path.Join("src", base)); err != nil {
			logger.LogWarning(err.Error())
		}
	}

	totalBytes := playlist.GetTotalSize(config.Destination, existingSize)
	playlist.Size = totalBytes - existingSize

	logger.LogInfo("Starting conversion")

	start := time.Now().Add(-time.Second)

	// loop through each playlist item
	for i := 0; i < len(playlistItems); i++ {
		displayCloneProcessStatus(*playlist, existingSize, totalBytes, start)

		item := playlistItems[i]
		var key = plex.GetKey(item.Path)

		var originalBytes int64
		if existing[key] > 0 {
			originalBytes = 0
		} else {
			originalBytes = int64(src.GetSize(item.Path))
		}

		// if the remaining size available for the playlist is less than the size of the item, remove any extraneous
		// items from the dest directory until the size needed to copy the item is available
		if playlist.Size < originalBytes && i != len(playlistItems)-1 {
			logger.LogInfo("Making space - Checking if any of the", len(playlistItems)-i, "lower priority items exist")
			toRemove := originalBytes - playlist.Size
			clearedBytes := removeLast(toRemove, dest, playlistItems[i+1:], existing)
			if clearedBytes > 0 {
				logger.LogInfo("Removed last", humanize.Bytes(uint64(clearedBytes)), " from ", playlist.Name)
				playlist.Size += clearedBytes
				displayCloneProcessStatus(*playlist, existingSize, totalBytes, start)
			}
		}
		if existing[key] > 0 {
			logger.LogInfo("File exists, skipping ", item.Path)
			continue
		}

		if playlist.Size < humanize.MiByte*50 {
			logger.LogInfo(logger.Green, "Finished copying ", playlist.Name, logger.Reset)
			playlist.Size = 0
			playlist.Items = []PlaylistItem{}
			break
		}

		destFile, size, err := reencodeVideo(playlist.Name, src, dest, item.Path, config.TempDir, fast)
		if err != nil {
			logger.LogErrorf("Error reencoding %s: %s\n", item.Path, err)
			continue
		}

		if int64(size) > playlist.Size {
			_ = destFile.Remove()
			logger.LogInfo(logger.Green, "Finished copying ", playlist.Name, logger.Reset)
			playlist.Size = 0
			playlist.Items = []PlaylistItem{}
			break
		}

		playlist.Size = int64(math.Max(float64(playlist.Size)-float64(size), 0))
		playlist.Size, totalBytes = checkFreeSpace(config.Destination, playlist, totalBytes)
		playlist.Items = playlistItems[i+1:]

		progress <- playlist
		logger.LogVerbose("Moving to next item")
	}
	wg.Done()
}

func displayCloneProcessStatus(playlist Playlist, existingSize int64, totalBytes int64, start time.Time) {
	if (logger.LogLevel != "WARN") && (logger.LogLevel != "ERROR") {
		// this display label can be incorrect, but it doesn't really matter for processing the playlist
		usedSize := totalBytes - playlist.Size + 1
		remaining := time.Duration((float64(time.Since(start)) / float64(usedSize-existingSize)) * float64(playlist.Size))
		if remaining < 0 { // this should be enough to check invalid times
			remaining = time.Duration(3*len(playlist.Items)) * time.Minute // rough estimate
		}

		logger.LogPersistent(playlist.Name, logger.Green, playlist.Name, ": ",
			humanize.Bytes(uint64(usedSize)), " used of ", humanize.Bytes(uint64(totalBytes)), ", ",
			time.Since(start).Round(time.Second).String(), " elapsed, ",
			remaining.Round(time.Second).String(), " (", humanizeNegBytes(playlist.Size), ") remaining.",
			logger.Reset, "\n")
	}
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

// Remove items from the end of the playlist until we have enough space to copy the next file
func removeLast(bytes int64, dest FileSystem, items []PlaylistItem, existing map[string]uint64) int64 {
	removed := int64(0)
	for i := len(items) - 1; i >= 0; i-- {
		key := plex.GetKey(items[i].Path)
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

func checkFreeSpace(root string, playlist *Playlist, totalBytes int64) (int64, int64) {
	fs := NewFileSystem(root)
	size, err := fs.GetFreeSpace(playlist.GetBase())
	if err != nil {
		return playlist.Size, totalBytes
	}
	if size < uint64(playlist.Size) {
		logger.LogWarning("Not enough free space, adjusting size from", humanize.Bytes(uint64(playlist.Size)), "to", humanize.Bytes(size))
		return int64(size) - (500 * humanize.MiByte), totalBytes - (playlist.Size - int64(size))
	}
	// leave some space on the drive
	return playlist.Size, totalBytes
}
