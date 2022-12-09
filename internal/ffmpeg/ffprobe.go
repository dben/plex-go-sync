package ffmpeg

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os/exec"
	"plex-go-sync/internal/filesystem"
	"plex-go-sync/internal/logger"
	"plex-go-sync/internal/models"
	"strconv"
	"time"
)

type probeFormat struct {
	Duration   string `json:"duration"`
	FormatName string `json:"format_name"`
	BitRate    string `json:"bit_rate"`
	Size       string `json:"size"`
}

type probeStreams struct {
	CodecName  string `json:"codec_name"`
	CodecType  string `json:"codec_type"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	BitRate    string `json:"bit_rate"`
	Duration   string `json:"duration"`
	DurationTs int    `json:"duration_ts"`
	TimeBase   string `json:"time_base"`
	Tags       struct {
		BPS string `json:"BPS"`
	}
}

type probePackets struct {
	DurationTime string `json:"duration_time"`
	DtsTime      string `json:"dts_time"`
}

type probeData struct {
	Format  probeFormat    `json:"format"`
	Streams []probeStreams `json:"streams"`
	Packets []probePackets `json:"packets"`
}

func Probe(ctx *context.Context, file filesystem.File, size uint64) (ok bool, duration time.Duration, audioStreams []int, err error) {
	var config = models.GetConfig(ctx)
	str, s2, err := callProbe(ctx, file, "-show_format", "-show_streams")
	if err != nil {
		logger.LogWarning("Error while probing file: %s", err.Error())
		return false, 0, []int{}, err
	}
	if size == 0 {
		size = uint64(s2)
	}
	duration, _, bitrate, height, audioStreams, err := getProbeData(str, size)

	logger.LogVerbose(file.GetRelativePath(), " - duration=", duration, ", bitrate=", bitrate, ", height=", height)

	if err == nil &&
		bitrate > 0 &&
		height > 0 &&
		bitrate <= config.MediaFormat.BitrateFilter &&
		height <= config.MediaFormat.HeightFilter {
		return true, duration, audioStreams, nil
	}

	return false, duration, audioStreams, err
}

func ProbeActualDuration(ctx *context.Context, file filesystem.File) (duration time.Duration, err error) {
	str, _, err := callProbe(ctx, file, "-show_entries", "packet=duration_time,dts_time", "-read_intervals", "999999", "-select_streams", "a")
	if err != nil {
		logger.LogWarning("Error while probing file: %s", err.Error())
		return 0, err
	}
	pd := probeData{}
	err = json.Unmarshal([]byte(str), &pd)
	var seconds float64 = 0
	for _, packet := range pd.Packets {
		dur, err := strconv.ParseFloat(packet.DurationTime, 64)
		if err != nil {
			continue
		}
		dts, err := strconv.ParseFloat(packet.DtsTime, 64)
		if err != nil {
			continue
		}
		if dts+dur > seconds {
			seconds = dts + dur
		}
	}

	return time.Duration(seconds * float64(time.Second)), err
}

func getProbeData(result string, statSize uint64) (duration time.Duration, size uint64, bitrate int, height int, audioStreams []int, err error) {
	pd := probeData{}
	err = json.Unmarshal([]byte(result), &pd)

	if err != nil {
		return 0, 0, 0, 0, []int{}, err
	}

	durationSec, err := strconv.ParseFloat(pd.Format.Duration, 64)
	if err != nil {
		durationSec = 0
	}

	size, err = strconv.ParseUint(pd.Format.Size, 10, 64)
	if err != nil || size == 0 {
		size = statSize
	}

	height = 0
	var i = 0
	for _, stream := range pd.Streams {
		if stream.CodecType == "video" {
			bitrate, err := strconv.ParseUint(stream.BitRate, 10, 64)
			if (bitrate == 0) && stream.Tags.BPS != "" {
				bitrate, err = strconv.ParseUint(stream.Tags.BPS, 10, 64)
			}
			if err != nil || stream.Height == 0 {
				continue
			}
			height = stream.Height
			break
		}
		if stream.CodecType == "audio" {
			if stream.CodecName == "aac" {
				audioStreams = append(audioStreams, i)
			}
			i++
		}
	}

	if bitrate == 0 && durationSec > 0 {
		bitrate = int(float64(size) * 8 / durationSec)
	}

	return time.Duration(durationSec * float64(time.Second)), size, bitrate, height, audioStreams, nil
}

func callProbe(ctx *context.Context, file filesystem.File, args ...string) (string, int64, error) {
	var cmd *exec.Cmd
	var reader io.ReadCloser
	var err error
	if !file.IsLocal() {
		reader, err = file.ReadFile()
		if err != nil {
			_ = reader.Close()
			return "", 0, err
		}
		args = append(args, "-print_format", "json", "-loglevel", "warning", "-hide_banner", "pipe:0")
		cmd = exec.CommandContext(*ctx, "ffprobe", args...)
		cmd.Stdin = reader
	} else {
		args = append(args, "-print_format", "json", "-loglevel", "warning", "-hide_banner", file.GetAbsolutePath())
		cmd = exec.CommandContext(*ctx, "ffprobe", args...)
	}

	buf := bytes.NewBuffer(nil)
	re, err := cmd.StdoutPipe()
	var size int64 = 0
	if err == nil {
		err = cmd.Start()
	}
	if err == nil {
		size, err = buf.ReadFrom(re)
	}
	if err == nil {
		err = cmd.Wait()
	}
	if reader != nil {
		_ = reader.Close()
	}
	result := buf.String()
	return result, size, nil
}
