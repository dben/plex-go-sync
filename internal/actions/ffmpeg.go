package actions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/dustin/go-humanize"
	ffmpeg_go "github.com/u2takey/ffmpeg-go"
	"io"
	"log"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"path"
	"plex-go-sync/internal/filesystem"
	"plex-go-sync/internal/logger"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type FfmpegProps struct {
	Frame      uint64
	Fps        float64
	Bitrate    string
	TotalSize  uint64
	OutTime    time.Duration
	DupFrames  uint64
	DropFrames uint64
	Speed      string
	Progress   string
	Duration   time.Duration
	Elapsed    time.Duration
}

type probeFormat struct {
	Duration   string `json:"duration"`
	FormatName string `json:"format_name"`
	BitRate    string `json:"bit_rate"`
	Size       string `json:"size"`
}

type probeStreams struct {
	CodecName string `json:"codec_name"`
	CodecType string `json:"codec_type"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	BitRate   string `json:"bit_rate"`
	Tags      struct {
		BPS string `json:"BPS"`
	}
}

type probeData struct {
	Format  probeFormat    `json:"format"`
	Streams []probeStreams `json:"streams"`
}

type InputBufferError struct {
	message string
}
type OutputBufferError struct {
	message string
}

func (e *OutputBufferError) Error() string {
	return e.message
}

func (e *InputBufferError) Error() string {
	return e.message
}

const bitrateFilter = 3500 * humanize.KByte
const heightFilter = 720
const widthFilter = 1280
const crfFilter = 23
const MediaFormat = "mp4"
const defaultDuration = time.Hour * 4

func Ffprobe(reader io.Reader) (bool, time.Duration, []int, error) {
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "ffprobe", "-show_format", "-show_streams",
		"-print_format", "json", "pipe:0")
	cmd.Stdin = reader
	buf := bytes.NewBuffer(nil)
	re, err := cmd.StdoutPipe()
	if err != nil {
		return false, defaultDuration, []int{}, err
	}
	if err = cmd.Start(); err != nil {
		logger.LogWarning(buf.String())
		logger.LogWarning("Error:", err)
		return false, defaultDuration, []int{}, err
	}

	size, err := buf.ReadFrom(re)
	if err != nil {
		return false, defaultDuration, []int{}, err
	}
	if err = cmd.Wait(); err != nil {
		return false, defaultDuration, []int{}, err
	}

	if duration, _, bitrate, height, audioStreams, err := getProbeData(buf.String(), uint64(size)); err == nil && ((bitrate < bitrateFilter) && (height < heightFilter)) {
		return true, duration, audioStreams, nil
	}
	return false, defaultDuration, []int{}, err
}
func FfmpegConverter(in filesystem.File, out filesystem.File, duration time.Duration, kwargs ffmpeg_go.KwArgs) (chan FfmpegProps, chan error) {
	msg := make(chan error)
	uri, socket := ffmpegProgressSock(duration)

	go func() {
		defer close(msg)
		logger.LogInfo("ffmpeg", "Starting ffmpeg copy")
		buf := bytes.NewBuffer(nil)

		var cmd *ffmpeg_go.Stream

		if !in.IsLocal() {
			cmd = ffmpeg_go.Input("pipe:0")
		} else {
			cmd = ffmpeg_go.Input(path.Clean(in.GetAbsolutePath()))
		}

		if !out.IsLocal() {
			cmd = cmd.Output("pipe:1", kwargs)
		} else {
			cmd = cmd.Output(out.GetAbsolutePath(), kwargs)
		}

		cmd = cmd.GlobalArgs("-progress", uri).
			WithErrorOutput(buf)

		if !in.IsLocal() {
			reader, err := in.ReadFile()
			if err != nil {
				logger.LogWarning(err)
				msg <- err
				_ = reader.Close()
				return
			}
			logger.LogVerbose("Reading from io.Reader")
			cmd = cmd.WithInput(reader)
		}

		if !out.IsLocal() {
			writer, err := out.FileWriter()
			if err != nil {
				logger.LogWarning(err)
				msg <- err
				_ = writer.Close()
				return
			}
			cmd = cmd.WithOutput(writer)
		}

		err := cmd.Run()

		if err != nil {
			if strings.Contains(buf.String(), "muxer does not support non seekable input") {
				msg <- &InputBufferError{message: "muxer does not support non seekable input"}
			}
			if strings.Contains(buf.String(), "Cannot write moov atom") {
				msg <- &OutputBufferError{message: "cannot write moov atom"}
			}
			if strings.Contains(buf.String(), "muxer does not support non seekable output") {
				msg <- &OutputBufferError{message: "muxer does not support non seekable output"}
			}
			logger.LogWarning(buf.String(), err)
			msg <- err
		}
	}()
	return socket, msg
}

func getProbeData(result string, statSize uint64) (time.Duration, uint64, uint64, int, []int, error) {
	pd := probeData{}
	err := json.Unmarshal([]byte(result), &pd)
	if err != nil {
		return 0, 0, 0, 0, []int{}, err
	}
	duration, err := strconv.ParseFloat(pd.Format.Duration, 64)
	if err != nil {
		return 0, 0, 0, 0, []int{}, err
	}
	size, _ := strconv.ParseUint(pd.Format.Size, 10, 64)
	if size == 0 {
		size = statSize
	}

	var bitrate uint64
	var height = 0
	var i = 0
	var audioStreams []int
	for _, stream := range pd.Streams {
		if stream.CodecType == "video" {
			bitrate, err = strconv.ParseUint(stream.BitRate, 10, 64)
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

	if bitrate == 0 {
		bitrate = uint64(float64(size) * 8 / duration)
	}

	return time.Duration(duration * float64(time.Second)), size, bitrate, height, audioStreams, nil
}

func ffmpegProgressSock(duration time.Duration) (string, chan FfmpegProps) {
	rand.Seed(time.Now().Unix())
	var sockFileName string
	var l net.Listener
	for {
		var err error
		sockFileName = path.Join(os.TempDir(), fmt.Sprintf("%d_sock", rand.Int()))
		l, err = net.Listen("unix", sockFileName)
		if err == nil {
			break
		}
		if err != nil && !strings.Contains(err.Error(), "address already in use") {
			panic(err)
		}
		logger.LogWarning(err, sockFileName)
	}

	ch := make(chan FfmpegProps)

	go func() {
		Frame := regexp.MustCompile(`frame=(\d+)`)
		Fps := regexp.MustCompile(`fps=([\d.]+)`)
		Bitrate := regexp.MustCompile(`bitrate=(\w+)`)
		TotalSize := regexp.MustCompile(`total_size=(\d+)`)
		OutTime := regexp.MustCompile(`out_time_ms=(\d+)`)
		DupFrames := regexp.MustCompile(`dup_frames=(\d+)`)
		DropFrames := regexp.MustCompile(`drop_frames=(\d+)`)
		Speed := regexp.MustCompile(`speed=\s*(\w+)`)
		Progress := regexp.MustCompile(`progress=(\w+)`)
		start := time.Now()

		fd, err := l.Accept()
		if err != nil {
			log.Fatal("accept error:", err)
		}
		ot := time.Duration(0)
		buf := make([]byte, 16)
		data := ""
		for {
			_, err := fd.Read(buf)
			if err != nil {
				close(ch)
				_ = l.Close()
				return
			}
			data += string(buf)
			props := FfmpegProps{Duration: duration}
			if m := OutTime.FindAllStringSubmatch(data, -1); len(m) > 0 && len(m[len(m)-1]) > 1 {
				outTime, _ := strconv.ParseInt(m[len(m)-1][1], 10, 64)
				props.OutTime = time.Duration(outTime * int64(time.Microsecond))
				if props.OutTime == ot || props.OutTime == 0 {
					continue
				}
				ot = props.OutTime
			}

			if m := Frame.FindAllStringSubmatch(data, -1); len(m) > 0 && len(m[len(m)-1]) > 1 {
				props.Frame, _ = strconv.ParseUint(m[len(m)-1][1], 10, 64)
			}
			if m := Fps.FindAllStringSubmatch(data, -1); len(m) > 0 && len(m[len(m)-1]) > 1 {
				props.Fps, _ = strconv.ParseFloat(m[len(m)-1][1], 32)
			}
			if m := Bitrate.FindAllStringSubmatch(data, -1); len(m) > 0 && len(m[len(m)-1]) > 1 {
				props.Bitrate = m[len(m)-1][1]
			}
			if m := TotalSize.FindAllStringSubmatch(data, -1); len(m) > 0 && len(m[len(m)-1]) > 1 {
				props.TotalSize, _ = strconv.ParseUint(m[len(m)-1][1], 10, 64)
			}
			if m := DupFrames.FindAllStringSubmatch(data, -1); len(m) > 0 && len(m[len(m)-1]) > 1 {
				props.DupFrames, _ = strconv.ParseUint(m[len(m)-1][1], 10, 64)
			}
			if m := DropFrames.FindAllStringSubmatch(data, -1); len(m) > 0 && len(m[len(m)-1]) > 1 {
				props.DropFrames, _ = strconv.ParseUint(m[len(m)-1][1], 10, 64)
			}
			if m := Speed.FindAllStringSubmatch(data, -1); len(m) > 0 && len(m[len(m)-1]) > 1 {
				props.Speed = m[len(m)-1][1]
			}
			if m := Progress.FindAllStringSubmatch(data, -1); len(m) > 0 && len(m[len(m)-1]) > 1 {
				props.Progress = m[len(m)-1][1]
			}

			props.Elapsed = time.Since(start)

			ch <- props
		}
	}()

	return "unix://" + sockFileName, ch
}
