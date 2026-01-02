// Package logger provides a simple structured logging system for TechPulse.
package logger

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// Level represents the severity level of a log message.
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

// String returns the string representation of a log level.
func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// Logger is the interface for structured logging.
type Logger interface {
	Debug(msg string, fields ...Field)
	Info(msg string, fields ...Field)
	Warn(msg string, fields ...Field)
	Error(msg string, fields ...Field)
	WithFields(fields ...Field) Logger
}

// Field represents a key-value pair for structured logging.
type Field struct {
	Key   string
	Value any
}

// F creates a new Field with the given key and value.
func F(key string, value any) Field {
	return Field{Key: key, Value: value}
}

// defaultLogger is the package-level default logger instance.
var (
	defaultLogger Logger = NewLogger(os.Stdout, LevelInfo)
	mu            sync.RWMutex
)

// SetDefault sets the default package-level logger.
func SetDefault(l Logger) {
	mu.Lock()
	defer mu.Unlock()
	defaultLogger = l
}

// Default returns the default package-level logger.
func Default() Logger {
	mu.RLock()
	defer mu.RUnlock()
	return defaultLogger
}

// Debug logs a debug message using the default logger.
func Debug(msg string, fields ...Field) { Default().Debug(msg, fields...) }

// Info logs an info message using the default logger.
func Info(msg string, fields ...Field) { Default().Info(msg, fields...) }

// Warn logs a warning message using the default logger.
func Warn(msg string, fields ...Field) { Default().Warn(msg, fields...) }

// Error(msg string, fields ...Field) logs an error using the default logger.
func Error(msg string, fields ...Field) { Default().Error(msg, fields...) }

// formatFields converts fields to a string representation.
func formatFields(fields []Field) string {
	if len(fields) == 0 {
		return ""
	}
	var parts []string
	for _, f := range fields {
		parts = append(parts, fmt.Sprintf("%s=%v", f.Key, f.Value))
	}
	return " " + strings.Join(parts, " ")
}

// formatTime returns a formatted timestamp string.
func formatTime(t time.Time) string {
	return t.Format("15:04:05")
}
