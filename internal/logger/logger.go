package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strings"
	"sync"
)

const Yellow = "\033[0;33m"
const Green = "\033[0;32m"
const Blue = "\033[0;34m"
const Magenta = "\033[0;35m"
const Red = "\033[0;31m"
const Reset = "\033[0m"

var depth = 2
var stdout = log.New(os.Stdout, "", 0)
var stderr = log.New(os.Stderr, "", 0)
var LogLevel = "INFO"
var persistent = make(map[string]string)
var mutex sync.Mutex
var cleanup string

func SetLogLevel(level string) {
	r, w := io.Pipe()
	LogLevel = strings.ToUpper(level)
	if level == "VERBOSE" {
		stdout.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
		stderr.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	} else {
		log.SetFlags(0)
	}
	go func() {
		if LogLevel != "WARN" && LogLevel != "ERROR" {
			buffer := make([]byte, 1024)
			var s string
			for {
				n, err := r.Read(buffer)
				if err != nil {
					break
				}
				s += string(buffer[:n])
				idx := strings.Index(s, "\n")
				if n > 0 && idx > 0 {
					split := strings.Split(s, "\n")
					s = split[len(split)-1]
					for _, line := range split[:len(split)-1] {
						line = strings.TrimSpace(strings.ToValidUTF8(line, ""))
						if line != "" {
							if LogLevel != "WARN" && LogLevel != "ERROR" {
								Cleanup()
								fmt.Println(Magenta + line + Reset)
							}
						}
					}
				}
			}
		}
	}()

	log.SetOutput(w)
}

func LogVerbose(v ...any) {
	if LogLevel == "VERBOSE" {
		Cleanup()
		_ = stdout.Output(depth, fmt.Sprintln(v...))
	}
}
func LogInfo(v ...any) {
	if LogLevel != "WARN" && LogLevel != "ERROR" {
		Cleanup()
		_ = stdout.Output(depth, fmt.Sprintln(v...))
	}
}
func LogWarning(v ...any) {
	if LogLevel != "ERROR" {
		Cleanup()
		_ = stdout.Output(depth, Yellow+fmt.Sprintln(v...)+Reset)
	}
}
func LogError(v ...any) {
	Cleanup()
	_ = stderr.Output(depth, Red+fmt.Sprintln(v...)+Reset)
}

func LogVerbosef(format string, v ...any) {
	if LogLevel == "VERBOSE" {
		Cleanup()
		_ = stdout.Output(depth, fmt.Sprintf(format, v...))
	}
}
func LogInfof(format string, v ...any) {
	if LogLevel != "WARN" && LogLevel != "ERROR" {
		Cleanup()
		_ = stdout.Output(depth, fmt.Sprintf(format, v...))
	}
}
func LogWarningf(format string, v ...any) {
	if LogLevel != "ERROR" {
		Cleanup()
		_ = stdout.Output(depth, Yellow+fmt.Sprintf(format, v...)+Reset)
	}
}
func LogErrorf(format string, v ...any) {
	Cleanup()
	_ = stderr.Output(depth, Red+fmt.Sprintf(format, v...)+Reset)
}
func LogPersistent(id string, v ...any) {
	mutex.Lock()
	defer mutex.Unlock()
	if _, found := persistent[id]; len(v) == 0 && !found {
		return
	}

	Cleanup()
	if len(v) != 0 {
		persistent[id] = strings.TrimSuffix(fmt.Sprint(v...), "\n")
	} else {
		fmt.Println(persistent[id])
		delete(persistent, id)
	}
	keys := make([]string, len(persistent))
	i := 0
	for k := range persistent {

		if persistent[k] == "" {
			delete(persistent, k)
		} else {
			keys[i] = k
		}
		i++
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Println(persistent[k])
		cleanup += strings.Repeat(" ", len(persistent[k])) + "\n"
	}
	for range persistent {
		fmt.Print("\033[F")
		cleanup += "\033[F"
	}

}

func Progress(id string, percent float64, v ...any) {
	if LogLevel != "WARN" && LogLevel != "ERROR" {
		LogPersistent(id, fmt.Sprintf(Blue+"[%-50s]"+Reset+" %3d%% ", strings.Repeat("#", int(percent*50)), int(percent*100)), fmt.Sprint(v...))
	}
}
func ProgressClear(id string) {
	LogPersistent(id)
}
func Cleanup() {
	if cleanup != "" {
		fmt.Print(cleanup)
		cleanup = ""
	}
}
