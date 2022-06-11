package actions

import (
	"encoding/json"
	"fmt"
	ffmpeg_go "github.com/u2takey/ffmpeg-go"
	"log"
	"math/rand"
	"net"
	"os"
	"path"
	"plex-go-sync/internal/filesystem"
	"regexp"
	"strconv"
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
}

type probeData struct {
	Format probeFormat `json:"format"`
}

func FfmpegConverter(in filesystem.File, out filesystem.File, args ffmpeg_go.KwArgs) (chan FfmpegProps, chan error) {
	msg := make(chan error)
	result, _ := ffmpeg_go.Probe(path.Clean(in.GetAbsolutePath()))
	duration, err := getProbeDuration(result)
	if err != nil || duration == 0 {
		duration = 4 * time.Hour //default to 4 hours, should be bigger than needed
	}

	uri, socket := ffmpegProgressSock(duration)

	go func() {
		defer close(msg)
		if err != nil {
			msg <- err
		}

		fmt.Println("Converting: ", in.GetAbsolutePath())

		err = ffmpeg_go.Input(path.Clean(in.GetAbsolutePath())).
			Output(path.Clean(out.GetAbsolutePath()), args).
			GlobalArgs("-progress", uri).
			ErrorToStdOut().
			Run()

		if err != nil {
			msg <- err
		}
	}()

	return socket, msg
}

func getProbeDuration(a string) (time.Duration, error) {
	pd := probeData{}
	err := json.Unmarshal([]byte(a), &pd)
	if err != nil {
		return 0, err
	}
	f, err := strconv.ParseFloat(pd.Format.Duration, 64)
	if err != nil {
		return 0, err
	}
	return time.Duration(f * float64(time.Second)), nil
}

func ffmpegProgressSock(duration time.Duration) (string, chan FfmpegProps) {
	rand.Seed(time.Now().Unix())
	sockFileName := path.Join(os.TempDir(), fmt.Sprintf("%d_sock", rand.Int()))
	l, err := net.Listen("unix", sockFileName)
	if err != nil {
		panic(err)
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
				return
			}
			data += string(buf)
			props := FfmpegProps{Duration: duration}
			if m := OutTime.FindAllStringSubmatch(data, -1); len(m) > 0 && len(m[len(m)-1]) > 1 {
				outTime, _ := strconv.ParseInt(m[len(m)-1][1], 10, 64)
				props.OutTime = time.Duration(outTime * int64(time.Microsecond))
				if props.OutTime == ot {
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
