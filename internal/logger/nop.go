// Package logger provides a no-op logger for testing and silent mode.
package logger

// NopLogger is a logger that discards all log messages.
type NopLogger struct{}

// NewNopLogger creates a new NopLogger.
func NewNopLogger() *NopLogger {
	return &NopLogger{}
}

// Debug discards the message.
func (l *NopLogger) Debug(msg string, fields ...Field) {}

// Info discards the message.
func (l *NopLogger) Info(msg string, fields ...Field) {}

// Warn discards the message.
func (l *NopLogger) Warn(msg string, fields ...Field) {}

// Error discards the message.
func (l *NopLogger) Error(msg string, fields ...Field) {}

// WithFields returns itself (no fields to add).
func (l *NopLogger) WithFields(fields ...Field) Logger {
	return l
}
