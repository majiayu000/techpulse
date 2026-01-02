package techpulse

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/anthropic/autonomous-runner/internal/logger"
)

func TestConfigWatcherDetectsChange(t *testing.T) {
	// Create temp config file
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("limit: 10\n"), 0644); err != nil {
		t.Fatal(err)
	}

	changeCalled := make(chan *FileConfig, 1)
	w := NewConfigWatcher(WatcherConfig{
		Path:     path,
		Interval: 50 * time.Millisecond,
		OnChange: func(cfg *FileConfig) {
			changeCalled <- cfg
		},
	})
	w.SetLogger(logger.NewNopLogger())

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Start watching in background
	go func() {
		_ = w.Watch(ctx)
	}()

	// Wait a bit, then modify file
	time.Sleep(100 * time.Millisecond)
	if err := os.WriteFile(path, []byte("limit: 20\n"), 0644); err != nil {
		t.Fatal(err)
	}

	select {
	case cfg := <-changeCalled:
		if cfg.Limit != 20 {
			t.Errorf("expected limit 20, got %d", cfg.Limit)
		}
	case <-ctx.Done():
		t.Fatal("timeout waiting for config change")
	}
}

func TestConfigWatcherReturnsErrorForMissingFile(t *testing.T) {
	w := NewConfigWatcher(WatcherConfig{
		Path:     "/nonexistent/config.yaml",
		Interval: 50 * time.Millisecond,
	})
	w.SetLogger(logger.NewNopLogger())

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := w.Watch(ctx)
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestConfigWatcherLastModified(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("limit: 10\n"), 0644); err != nil {
		t.Fatal(err)
	}

	w := NewConfigWatcher(WatcherConfig{
		Path:     path,
		Interval: 50 * time.Millisecond,
	})
	w.SetLogger(logger.NewNopLogger())

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	go func() {
		_ = w.Watch(ctx)
	}()

	time.Sleep(100 * time.Millisecond)
	if w.LastModified().IsZero() {
		t.Error("lastMod should not be zero after starting watch")
	}
}

func TestConfigWatcherIgnoresInvalidConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("limit: 10\n"), 0644); err != nil {
		t.Fatal(err)
	}

	changeCalled := false
	w := NewConfigWatcher(WatcherConfig{
		Path:     path,
		Interval: 50 * time.Millisecond,
		OnChange: func(_ *FileConfig) {
			changeCalled = true
		},
	})
	w.SetLogger(logger.NewNopLogger())

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	go func() {
		_ = w.Watch(ctx)
	}()

	// Write invalid YAML
	time.Sleep(100 * time.Millisecond)
	if err := os.WriteFile(path, []byte("invalid: [yaml"), 0644); err != nil {
		t.Fatal(err)
	}

	time.Sleep(200 * time.Millisecond)
	if changeCalled {
		t.Error("onChange should not be called for invalid config")
	}
}

func TestDefaultWatcherInterval(t *testing.T) {
	w := NewConfigWatcher(WatcherConfig{
		Path: "test.yaml",
	})
	if w.interval != DefaultWatcherInterval {
		t.Errorf("expected %v, got %v", DefaultWatcherInterval, w.interval)
	}
}
