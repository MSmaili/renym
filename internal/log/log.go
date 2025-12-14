package log

import (
	"fmt"
	"io"
	"os"
)

type Level int

const (
	// LevelSilent suppresses all output except errors
	LevelSilent Level = iota
	// LevelNormal shows user-facing messages (default)
	LevelNormal
	// LevelDebug shows verbose/debug messages
	LevelDebug
)

type Logger struct {
	level  Level
	out    io.Writer
	errOut io.Writer
}

// std is the default logger instance
var std = &Logger{
	level:  LevelNormal,
	out:    os.Stdout,
	errOut: os.Stderr,
}

func SetLevel(l Level) {
	std.level = l
}

func GetLevel() Level {
	return std.level
}

// SetOutput sets the output destination for the default logger
func SetOutput(w io.Writer) {
	std.out = w
}

// SetErrorOutput sets the error output destination for the default logger
func SetErrorOutput(w io.Writer) {
	std.errOut = w
}

// Info prints user-facing messages (normal level and above)
func Info(format string, args ...any) {
	if std.level >= LevelNormal {
		fmt.Fprintf(std.out, format, args...)
	}
}

// Debug prints verbose/debug messages (debug level only)
func Debug(format string, args ...any) {
	if std.level >= LevelDebug {
		fmt.Fprintf(std.out, format, args...)
	}
}

// Warn prints warnings to stderr (normal level and above)
func Warn(format string, args ...any) {
	if std.level >= LevelNormal {
		fmt.Fprintf(std.errOut, "Warning: "+format, args...)
	}
}

// Error prints errors to stderr (always, unless silent)
func Error(format string, args ...any) {
	if std.level > LevelSilent {
		fmt.Fprintf(std.errOut, format, args...)
	}
}

// Print prints to stdout regardless of level (for results that must always show)
func Print(format string, args ...any) {
	fmt.Fprintf(std.out, format, args...)
}
