// Package techpulse daemon mode integration tests.
package techpulse

import (
	"context"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/anthropic/autonomous-runner/internal/collector"
	"github.com/anthropic/autonomous-runner/internal/collector/hackernews"
	"github.com/anthropic/autonomous-runner/internal/filter"
	"github.com/anthropic/autonomous-runner/internal/logger"
	"github.com/anthropic/autonomous-runner/internal/storage"
	"github.com/anthropic/autonomous-runner/internal/summarizer"
)

func TestDaemonRunsImmediately(t *testing.T) {
	factory, cleanup := createTestFactory(t)
	defer cleanup()

	var collections int32
	daemon := NewDaemon(Config{}, DaemonConfig{
		Interval:         100 * time.Millisecond,
		TechPulseFactory: factory,
		OnDone: func() {
			atomic.AddInt32(&collections, 1)
		},
	})
	daemon.SetLogger(logger.NewNopLogger())

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	daemon.Run(ctx)
	if atomic.LoadInt32(&collections) < 1 {
		t.Error("daemon should run collection immediately on start")
	}
}

func TestDaemonMultipleCollections(t *testing.T) {
	factory, cleanup := createTestFactory(t)
	defer cleanup()

	var collections int32
	daemon := NewDaemon(Config{}, DaemonConfig{
		Interval:         200 * time.Millisecond,
		TechPulseFactory: factory,
		OnDone: func() {
			atomic.AddInt32(&collections, 1)
		},
	})
	daemon.SetLogger(logger.NewNopLogger())

	// Allow ample time for 2+ collections
	ctx, cancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
	defer cancel()

	count, _ := daemon.Run(ctx)
	if count < 2 {
		t.Errorf("expected at least 2 collections, got %d", count)
	}
}

func TestDaemonContextCancellation(t *testing.T) {
	factory, cleanup := createTestFactory(t)
	defer cleanup()

	var stopped bool
	daemon := NewDaemon(Config{}, DaemonConfig{
		Interval:         1 * time.Hour,
		TechPulseFactory: factory,
		OnStop: func() {
			stopped = true
		},
	})
	daemon.SetLogger(logger.NewNopLogger())

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		daemon.Run(ctx)
		close(done)
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("daemon did not stop after context cancellation")
	}

	if !stopped {
		t.Error("OnStop callback was not called")
	}
}

func TestDaemonCallbacks(t *testing.T) {
	factory, cleanup := createTestFactory(t)
	defer cleanup()

	var (
		started int32
		ticked  int32
		done    int32
		stopped int32
	)

	daemon := NewDaemon(Config{}, DaemonConfig{
		Interval:         50 * time.Millisecond,
		TechPulseFactory: factory,
		OnStart:          func() { atomic.AddInt32(&started, 1) },
		OnTick:           func() { atomic.AddInt32(&ticked, 1) },
		OnDone:           func() { atomic.AddInt32(&done, 1) },
		OnStop:           func() { atomic.AddInt32(&stopped, 1) },
	})
	daemon.SetLogger(logger.NewNopLogger())

	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()

	daemon.Run(ctx)

	if atomic.LoadInt32(&started) != 1 {
		t.Error("OnStart should be called exactly once")
	}
	if atomic.LoadInt32(&stopped) != 1 {
		t.Error("OnStop should be called exactly once")
	}
	if atomic.LoadInt32(&ticked) < 1 {
		t.Error("OnTick should be called at least once")
	}
	if atomic.LoadInt32(&done) < 1 {
		t.Error("OnDone should be called at least once")
	}
}

func TestDaemonReturnsCollectionCount(t *testing.T) {
	factory, cleanup := createTestFactory(t)
	defer cleanup()

	daemon := NewDaemon(Config{}, DaemonConfig{
		Interval:         200 * time.Millisecond,
		TechPulseFactory: factory,
	})
	daemon.SetLogger(logger.NewNopLogger())

	ctx, cancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
	defer cancel()

	count, err := daemon.Run(ctx)
	if err != context.DeadlineExceeded {
		t.Errorf("expected DeadlineExceeded error, got %v", err)
	}
	if count < 2 {
		t.Errorf("expected at least 2 successful collections, got %d", count)
	}
}

func TestDefaultDaemonConfig(t *testing.T) {
	cfg := DefaultDaemonConfig()
	if cfg.Interval != 24*time.Hour {
		t.Errorf("expected 24h default interval, got %v", cfg.Interval)
	}
}

func TestDaemonWithConfigProvider(t *testing.T) {
	factory, cleanup := createTestFactory(t)
	defer cleanup()

	var currentLimit int32 = 5
	var providerCalls int32

	daemon := NewDaemon(Config{Limit: 5}, DaemonConfig{
		Interval:         50 * time.Millisecond,
		TechPulseFactory: factory,
		ConfigProvider: func() Config {
			atomic.AddInt32(&providerCalls, 1)
			return Config{
				Limit: int(atomic.LoadInt32(&currentLimit)),
			}
		},
	})
	daemon.SetLogger(logger.NewNopLogger())

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	daemon.Run(ctx)

	if atomic.LoadInt32(&providerCalls) < 2 {
		t.Errorf("ConfigProvider should be called at least twice, got %d", providerCalls)
	}
}

// createTestFactory creates a factory that returns TechPulse with mock servers.
func createTestFactory(t *testing.T) (func(Config) *TechPulse, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "techpulse-daemon-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	hnServer := createMockHNServer(t)

	reg := collector.NewRegistry()
	hnClient := hackernews.NewClientWithBaseURL(hnServer.URL)
	reg.Register(hackernews.NewWithClient("top", hnClient))

	storeCfg := storage.DefaultConfig()
	storeCfg.BaseDir = tmpDir

	factory := func(cfg Config) *TechPulse {
		return &TechPulse{
			config:     Config{Limit: 5, Timeout: 10},
			registry:   reg,
			pipeline:   filter.DefaultPipeline(),
			summarizer: summarizer.NewBasicSummarizer(),
			storage:    storage.NewMarkdownStorage(storeCfg),
			log:        logger.NewNopLogger(),
		}
	}

	cleanup := func() {
		hnServer.Close()
		os.RemoveAll(tmpDir)
	}

	return factory, cleanup
}
