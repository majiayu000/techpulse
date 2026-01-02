package logger

import (
	"bytes"
	"strings"
	"testing"
)

func TestLevelString(t *testing.T) {
	tests := []struct {
		level Level
		want  string
	}{
		{LevelDebug, "DEBUG"},
		{LevelInfo, "INFO"},
		{LevelWarn, "WARN"},
		{LevelError, "ERROR"},
		{Level(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		if got := tt.level.String(); got != tt.want {
			t.Errorf("Level(%d).String() = %q, want %q", tt.level, got, tt.want)
		}
	}
}

func TestFieldCreation(t *testing.T) {
	f := F("key", "value")
	if f.Key != "key" || f.Value != "value" {
		t.Errorf("F() = %+v, want Key=key, Value=value", f)
	}
}

func TestFormatFields(t *testing.T) {
	tests := []struct {
		name   string
		fields []Field
		want   string
	}{
		{"empty", nil, ""},
		{"single", []Field{F("key", "val")}, " key=val"},
		{"multiple", []Field{F("a", 1), F("b", 2)}, " a=1 b=2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatFields(tt.fields); got != tt.want {
				t.Errorf("formatFields() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStandardLoggerOutput(t *testing.T) {
	buf := &bytes.Buffer{}
	log := NewLogger(buf, LevelDebug)

	log.Info("test message", F("key", "value"))

	output := buf.String()
	if !strings.Contains(output, "[INFO]") {
		t.Errorf("output missing [INFO]: %s", output)
	}
	if !strings.Contains(output, "test message") {
		t.Errorf("output missing message: %s", output)
	}
	if !strings.Contains(output, "key=value") {
		t.Errorf("output missing field: %s", output)
	}
}

func TestStandardLoggerLevelFilter(t *testing.T) {
	buf := &bytes.Buffer{}
	log := NewLogger(buf, LevelWarn)

	log.Debug("debug")
	log.Info("info")
	log.Warn("warn")
	log.Error("error")

	output := buf.String()
	if strings.Contains(output, "debug") {
		t.Error("debug should be filtered")
	}
	if strings.Contains(output, "[INFO]") {
		t.Error("info should be filtered")
	}
	if !strings.Contains(output, "warn") {
		t.Error("warn should appear")
	}
	if !strings.Contains(output, "error") {
		t.Error("error should appear")
	}
}

func TestWithFields(t *testing.T) {
	buf := &bytes.Buffer{}
	log := NewLogger(buf, LevelInfo)

	childLog := log.WithFields(F("ctx", "test"))
	childLog.Info("message")

	if !strings.Contains(buf.String(), "ctx=test") {
		t.Errorf("child logger missing context field: %s", buf.String())
	}
}

func TestSetDefaultLogger(t *testing.T) {
	buf := &bytes.Buffer{}
	newLog := NewLogger(buf, LevelInfo)

	oldDefault := Default()
	SetDefault(newLog)
	defer SetDefault(oldDefault)

	Info("test")
	if !strings.Contains(buf.String(), "test") {
		t.Error("default logger not used")
	}
}

func TestNopLogger(t *testing.T) {
	nop := NewNopLogger()

	// Should not panic
	nop.Debug("test")
	nop.Info("test")
	nop.Warn("test")
	nop.Error("test")
	nop.WithFields(F("key", "value")).Info("test")
}

func TestSetLevel(t *testing.T) {
	buf := &bytes.Buffer{}
	log := NewLogger(buf, LevelError)

	log.Info("should not appear")
	if buf.Len() > 0 {
		t.Error("message should be filtered at ERROR level")
	}

	log.SetLevel(LevelInfo)
	log.Info("should appear")
	if !strings.Contains(buf.String(), "should appear") {
		t.Error("message should appear after level change")
	}
}
