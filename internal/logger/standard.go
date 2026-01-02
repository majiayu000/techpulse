// Package logger provides standard logger implementation.
package logger

import (
	"fmt"
	"io"
	"sync"
	"time"
)

// StandardLogger is a simple structured logger implementation.
type StandardLogger struct {
	out    io.Writer
	level  Level
	fields []Field
	mu     sync.Mutex
}

// NewLogger creates a new StandardLogger.
func NewLogger(out io.Writer, level Level) *StandardLogger {
	return &StandardLogger{
		out:   out,
		level: level,
	}
}

// Debug logs a debug message.
func (l *StandardLogger) Debug(msg string, fields ...Field) {
	l.log(LevelDebug, msg, fields...)
}

// Info logs an info message.
func (l *StandardLogger) Info(msg string, fields ...Field) {
	l.log(LevelInfo, msg, fields...)
}

// Warn logs a warning message.
func (l *StandardLogger) Warn(msg string, fields ...Field) {
	l.log(LevelWarn, msg, fields...)
}

// Error logs an error message.
func (l *StandardLogger) Error(msg string, fields ...Field) {
	l.log(LevelError, msg, fields...)
}

// WithFields returns a new logger with additional fields.
func (l *StandardLogger) WithFields(fields ...Field) Logger {
	newFields := make([]Field, len(l.fields)+len(fields))
	copy(newFields, l.fields)
	copy(newFields[len(l.fields):], fields)

	return &StandardLogger{
		out:    l.out,
		level:  l.level,
		fields: newFields,
	}
}

// log writes a log message if the level is sufficient.
func (l *StandardLogger) log(level Level, msg string, fields ...Field) {
	if level < l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	allFields := append(l.fields, fields...)
	timestamp := formatTime(time.Now())
	levelStr := levelIcon(level)
	fieldStr := formatFields(allFields)

	fmt.Fprintf(l.out, "%s %s %s%s\n", timestamp, levelStr, msg, fieldStr)
}

// levelIcon returns a visual indicator for the log level.
func levelIcon(level Level) string {
	switch level {
	case LevelDebug:
		return "[DEBUG]"
	case LevelInfo:
		return "[INFO] "
	case LevelWarn:
		return "[WARN] "
	case LevelError:
		return "[ERROR]"
	default:
		return "[?????]"
	}
}

// SetLevel changes the minimum log level.
func (l *StandardLogger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// Level returns the current log level.
func (l *StandardLogger) Level() Level {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.level
}
