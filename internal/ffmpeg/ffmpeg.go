package ffmpeg

import (
	"bytes"
	"context"
	"fmt"
	ffmpeg_go "github.com/u2takey/ffmpeg-go"
	"log"
	"math/rand"
	"net"
	"os"
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

func Convert(ctx *context.Context, in filesystem.File, out filesystem.File, duration time.Duration, kwargs ffmpeg_go.KwArgs) (chan FfmpegProps, chan error) {
	msg := make(chan error)
	uri, socket := progressSocket(duration)
	logger.LogVerbose("Try ffmpeg convert: ", out.GetAbsolutePath())

	go func() {
		defer close(msg)
		buf := bytes.NewBuffer(nil)

		var cmd *ffmpeg_go.Stream

		if !in.IsLocal() {
			cmd = ffmpeg_go.Input("pipe:0")
		} else {
			cmd = ffmpeg_go.Input(path.Clean(in.GetAbsolutePath()))
		}

		cmd.Context = *ctx

		if !out.IsLocal() {
			kwargs["movflags"] = "frag_keyframe+empty_moov"
			cmd = cmd.Output("pipe:1", kwargs)
		} else {
			kwargs["movflags"] = "faststart"
			err := out.Mkdir()
			if err != nil {
				msg <- err
				return
			}
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
			cmd = cmd.WithInput(reader)
		}

		if !out.IsLocal() {
			writer, err := out.FileWriter()
			if err != nil {
				logger.LogWarning(err)
				msg <- err
				if writer != nil {
					_ = writer.Close()
				}
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
			if strings.Contains(buf.String(), "codec not currently supported in container") {
				msg <- &OutputBufferError{message: "codec not currently supported in container"}
			}
			if strings.Contains(err.Error(), "context canceled") {
				_ = out.Remove()
			}
			logger.LogWarning(buf.String(), err)
			msg <- err
		}
	}()
	return socket, msg
}

func progressSocket(duration time.Duration) (string, chan FfmpegProps) {
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
		Speed := regexp.MustCompile(`speed=\s*(\d+)x?`)
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
				if props.OutTime <= ot {
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
