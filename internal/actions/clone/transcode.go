package clone

import (
	"context"
	"errors"
	"fmt"
	"github.com/dustin/go-humanize"
	ffmpeg_go "github.com/u2takey/ffmpeg-go"
	"plex-go-sync/internal/ffmpeg"
	. "plex-go-sync/internal/filesystem"
	"plex-go-sync/internal/logger"
	"plex-go-sync/internal/models"
	"strconv"
	"strings"
	"time"
)

const fuzzySize = 20 * humanize.MiByte

// Transcode reencodes a video file to the correct format and copies it to the destination
func Transcode(ctx *context.Context, id string, src FileSystem, dest FileSystem, item models.PlaylistItem) (File, uint64, error) {
	var config = models.GetConfig(ctx)

	for _, srcPath := range item.Paths {
		srcFile := src.GetFile(srcPath)

		copyFile, duration, audioStreams, _ := ffmpeg.Probe(ctx, srcFile, 0)

		for _, destPath := range item.Paths {
			base, _ := getExtension(destPath)
			destFile := dest.GetFile(base + "." + config.MediaFormat.Format)
			size, err := tryConvert(ctx, srcFile, destFile, copyFile, duration, audioStreams, id+base)
			if err == nil {
				return destFile, size, err
			}
		}
	}

	return nil, 0, errors.New("could not transcode file")
}

func tryConvert(ctx *context.Context, srcFile File, destFile File, copyFile bool, duration time.Duration, audioStreams []int, id string) (uint64, error) {
	var config = models.GetConfig(ctx)
	var kwargs ffmpeg_go.KwArgs
	var isCorrectFormat = srcFile.GetExtension()[1:] == config.MediaFormat.Format
	var err error
	var totalSize uint64

	if copyFile && isCorrectFormat { // Just copy the file
		size, err := destFile.CopyFrom(ctx, srcFile.GetFileSystem(), id)
		return size, err
	} else if copyFile { // Do a format conversion
		kwargs = addAudioStream(ffmpeg_go.KwArgs{
			"vcodec":   "copy",
			"format":   config.MediaFormat.Format,
			"loglevel": "error", "y": "",
		}, audioStreams)
		progress, msg := ffmpeg.Convert(ctx, srcFile, destFile, duration, kwargs)
		totalSize, err = watchProgress(progress, msg, id)
		if err != nil && err.Error() == "codec not currently supported in container" {
			totalSize, err = tryConvert(ctx, srcFile, destFile, false, duration, audioStreams, id)
		}

	} else if config.FastConvert { // Skip this file
		return 0, errors.New("file must be converted, skipping because we are in -fast mode")
	} else { // Do a full re-encode
		kwargs = addAudioStream(ffmpeg_go.KwArgs{
			"c:v":      "libx264",
			"crf":      strconv.Itoa(config.MediaFormat.CrfFilter),
			"s":        fmt.Sprintf("%dx%d", config.MediaFormat.WidthFilter, config.MediaFormat.HeightFilter),
			"format":   config.MediaFormat.Format,
			"loglevel": "error", "y": "",
		}, audioStreams)
		progress, msg := ffmpeg.Convert(ctx, srcFile, destFile, duration, kwargs)
		totalSize, err = watchProgress(progress, msg, id)
	}

	if err == nil && totalSize <= 0 {
		err = errors.New("file has " + strconv.FormatUint(totalSize, 10) + " bytes")
		logger.LogWarning(err)
	}

	return totalSize, err
}

func addAudioStream(kwargs ffmpeg_go.KwArgs, audioStreams []int) ffmpeg_go.KwArgs {
	if len(audioStreams) == 0 {
		kwargs["c:a"] = "aac"
	} else {
		var maps = []string{"0:v"}
		for i := range audioStreams {
			maps = append(maps, "0:a:"+strconv.Itoa(i))
		}
		kwargs["map"] = maps
	}
	return kwargs
}

// watchProgress Display progress of the ffmpeg conversion
func watchProgress(progress chan ffmpeg.FfmpegProps, errChan chan error, id string) (fileSize uint64, err error) {
	totalSize := uint64(0)
	for {
		select {
		case data, more := <-progress:
			if more {
				percent := float64(data.OutTime) / float64(data.Duration)
				remaining := time.Duration((float64(data.Elapsed) / float64(data.OutTime+time.Second)) * float64(data.Duration-data.OutTime))
				idx := 0
				logger.Progress(id, percent, " at ", data.Speed+"x ", remaining.Round(time.Second).String(), " remaining \"", id[strings.IndexFunc(id, func(r rune) bool {
					if r == '/' {
						idx = idx + 1
					}
					return idx == 2
				}):][1:25]+"...\"")

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
