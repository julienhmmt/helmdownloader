// Package log provides a minimal levelled logger for helmdownloader.
package log

import (
	"fmt"
	"io"
	"sync"
	"time"
)

// Level controls how much detail is emitted.
type Level int

const (
	// LevelSilent suppresses all log output.
	LevelSilent Level = iota
	// LevelInfo emits informational messages.
	LevelInfo
	// LevelDebug emits detailed debug messages.
	LevelDebug
)

// String returns the human-readable name of a log level.
func (l Level) String() string {
	switch l {
	case LevelSilent:
		return "SILENT"
	case LevelInfo:
		return "INFO"
	case LevelDebug:
		return "DEBUG"
	default:
		return fmt.Sprintf("LEVEL(%d)", l)
	}
}

// Logger is a thread-safe levelled writer.
type Logger struct {
	mu     sync.Mutex
	writer io.Writer
	level  Level
}

// New creates a Logger that writes to w at the given level.
func New(w io.Writer, level Level) *Logger {
	if w == nil {
		w = io.Discard
	}
	return &Logger{writer: w, level: level}
}

// Discard returns a Logger that silently drops every message.
func Discard() *Logger {
	return New(io.Discard, LevelSilent)
}

// Infof writes an INFO line when the logger level is at least LevelInfo.
func (l *Logger) Infof(format string, args ...any) {
	l.logAt(LevelInfo, "INFO", format, args...)
}

// Debugf writes a DEBUG line when the logger level is at least LevelDebug.
func (l *Logger) Debugf(format string, args ...any) {
	l.logAt(LevelDebug, "DEBUG", format, args...)
}

// Errorf writes an ERROR line regardless of the logger level (unless silent).
func (l *Logger) Errorf(format string, args ...any) {
	l.logAt(LevelInfo, "ERROR", format, args...)
}

func (l *Logger) logAt(minLevel Level, prefix, format string, args ...any) {
	if l.level < minLevel {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	timestamp := time.Now().Format("2006-01-02T15:04:05.000Z07:00")
	line := fmt.Sprintf("[%s] %s %s\n", timestamp, prefix, fmt.Sprintf(format, args...))
	_, _ = io.WriteString(l.writer, line)
}
