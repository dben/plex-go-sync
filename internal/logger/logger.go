package logger

import (
	"fmt"
	"log"
	"os"
	"strings"
)

const Yellow = "\033[0;33m"
const Green = "\033[0;32m"
const Blue = "\033[0;34m"
const Red = "\033[0;31m"
const Off = "\033[0m"

var stdout = log.New(os.Stdout, "", 0)
var stderr = log.New(os.Stderr, "", 0)
var LogLevel = "INFO"

func SetLogLevel(level string) {
	LogLevel = strings.ToUpper(level)
	if level == "VERBOSE" {
		stdout.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
		stderr.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	} else {
		log.SetFlags(0)
	}
}

func LogVerbose(v ...any) {
	if LogLevel == "VERBOSE" {
		_ = stdout.Output(3, fmt.Sprintln(v...))
	}
}
func LogInfo(v ...any) {
	if LogLevel != "WARN" && LogLevel != "ERROR" {
		_ = stdout.Output(3, fmt.Sprintln(v...))
	}
}
func LogWarning(v ...any) {
	if LogLevel != "ERROR" {
		_ = stdout.Output(3, Yellow+fmt.Sprintln(v...)+Off)
	}
}
func LogError(v ...any) {
	_ = stderr.Output(3, Red+fmt.Sprintln(v...)+Off)
}

func LogVerbosef(format string, v ...any) {
	if LogLevel == "VERBOSE" {
		_ = stdout.Output(3, fmt.Sprintf(format, v...))
	}
}
func LogInfof(format string, v ...any) {
	if LogLevel != "WARN" && LogLevel != "ERROR" {
		_ = stdout.Output(3, fmt.Sprintf(format, v...))
	}
}
func LogWarningf(format string, v ...any) {
	if LogLevel != "ERROR" {
		_ = stdout.Output(3, Yellow+fmt.Sprintf(format, v...)+Off)
	}
}
func LogErrorf(format string, v ...any) {
	_ = stderr.Output(3, Red+fmt.Sprintf(format, v...)+Off)
}
func Progress(percent float64, v ...any) {
	if LogLevel != "WARN" && LogLevel != "ERROR" {
		fmt.Printf("\r"+Blue+"[%-50s]"+Off+" %3d%% %s          ", strings.Repeat("#", int(percent*50)), int(percent*100), fmt.Sprint(v...))
	}
}
