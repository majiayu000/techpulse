// Package techpulse daemon mode integration tests.
package techpulse

import (
	"context"
	"errors"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/majiayu000/techpulse/internal/collector"
	"github.com/majiayu000/techpulse/internal/collector/hackernews"
	"github.com/majiayu000/techpulse/internal/filter"
	"github.com/majiayu000/techpulse/internal/logger"
	"github.com/majiayu000/techpulse/internal/storage"
	"github.com/majiayu000/techpulse/internal/summarizer"
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
// Each produced instance gets a fresh collector/client, matching production,
// where every daemon tick builds a new registry (fresh HTTP clients and rate
// limiters) instead of sharing one across ticks.
func createTestFactory(t *testing.T) (func(Config) *TechPulse, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "techpulse-daemon-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	hnServer := createMockHNServer(t)

	storeCfg := storage.DefaultConfig()
	storeCfg.BaseDir = tmpDir

	factory := func(cfg Config) *TechPulse {
		reg := collector.NewRegistry()
		hnClient := hackernews.NewClientWithBaseURL(hnServer.URL)
		reg.Register(hackernews.NewWithClient("top", hnClient))
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

// newFakeDaemonFactory builds a TechPulseFactory around a single scripted
// collector writing into a throwaway directory.
func newFakeDaemonFactory(t *testing.T, c collector.Collector) func(Config) *TechPulse {
	t.Helper()
	storeCfg := storage.DefaultConfig()
	storeCfg.BaseDir = t.TempDir()
	return func(Config) *TechPulse {
		reg := collector.NewRegistry()
		reg.Register(c)
		return &TechPulse{
			config:     Config{Limit: 5},
			registry:   reg,
			pipeline:   filter.NewPipeline(),
			summarizer: summarizer.NewBasicSummarizer(),
			storage:    storage.NewMarkdownStorage(storeCfg),
			log:        logger.NewNopLogger(),
		}
	}
}

func alwaysOKCollector(name string) collector.Collector {
	return &fakeCollector{name: name, fn: func(_ context.Context, _ collector.Options) ([]collector.Article, error) {
		return []collector.Article{testArticle(1, "fine")}, nil
	}}
}

// TestDaemonRejectsInvalidInterval guards against the time.NewTicker panic on
// zero or negative intervals.
func TestDaemonRejectsInvalidInterval(t *testing.T) {
	tests := []struct {
		name     string
		interval time.Duration
		wantErr  bool
	}{
		{name: "zero interval", interval: 0, wantErr: true},
		{name: "negative interval", interval: -time.Hour, wantErr: true},
		{name: "positive interval runs", interval: 25 * time.Millisecond, wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var started bool
			daemon := NewDaemon(Config{}, DaemonConfig{
				Interval:             tt.interval,
				TechPulseFactory:     newFakeDaemonFactory(t, alwaysOKCollector("ok")),
				OnStart:              func() { started = true },
				InitialRetryDelay:    time.Millisecond,
				InitialRetryAttempts: -1, // no retries; keeps cases fast and isolated
			})
			daemon.SetLogger(logger.NewNopLogger())

			ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
			defer cancel()

			_, err := daemon.Run(ctx)
			if tt.wantErr {
				if err == nil || errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("Run() error = %v, want invalid-interval config error", err)
				}
				if started {
					t.Error("OnStart should not run when config is invalid")
				}
				return
			}
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("Run() error = %v, want DeadlineExceeded", err)
			}
		})
	}
}

// TestDaemonRetriesInitialCollectionBeforeInterval verifies a transient startup
// failure is retried quickly instead of idling for the full interval.
func TestDaemonRetriesInitialCollectionBeforeInterval(t *testing.T) {
	var calls, completed int32
	coll := &fakeCollector{name: "flaky", fn: func(_ context.Context, _ collector.Options) ([]collector.Article, error) {
		if atomic.AddInt32(&calls, 1) <= 2 {
			return nil, errors.New("transient outage")
		}
		return []collector.Article{testArticle(1, "recovered")}, nil
	}}

	daemon := NewDaemon(Config{}, DaemonConfig{
		Interval:          time.Hour, // without retries this would idle for an hour
		TechPulseFactory:  newFakeDaemonFactory(t, coll),
		InitialRetryDelay: 5 * time.Millisecond,
		OnDone: func() {
			atomic.AddInt32(&completed, 1)
		},
	})
	daemon.SetLogger(logger.NewNopLogger())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan int, 1)
	go func() {
		count, _ := daemon.Run(ctx)
		done <- count
	}()

	// Wait until the successful third attempt has completed (OnDone fires
	// only after a nil Run result) before cancelling.
	deadline := time.Now().Add(2 * time.Second)
	for atomic.LoadInt32(&completed) < 1 && time.Now().Before(deadline) {
		time.Sleep(2 * time.Millisecond)
	}
	cancel()

	select {
	case count := <-done:
		if atomic.LoadInt32(&calls) < 3 {
			t.Errorf("expected at least 3 collect attempts, got %d", calls)
		}
		if count < 1 {
			t.Errorf("expected at least 1 successful collection after retry, got %d", count)
		}
		if atomic.LoadInt32(&completed) != 1 {
			t.Errorf("OnDone should fire once on success, got %d", completed)
		}
	case <-time.After(time.Second):
		t.Fatal("daemon did not stop after cancellation")
	}
}

// TestDaemonInitialRetryDisabledByNegativeAttempts verifies InitialRetryAttempts
// < 0 disables initial-run retries entirely.
func TestDaemonInitialRetryDisabledByNegativeAttempts(t *testing.T) {
	var calls int32
	coll := &fakeCollector{name: "failing", fn: func(_ context.Context, _ collector.Options) ([]collector.Article, error) {
		atomic.AddInt32(&calls, 1)
		return nil, errors.New("always failing")
	}}

	daemon := NewDaemon(Config{}, DaemonConfig{
		Interval:             time.Hour,
		TechPulseFactory:     newFakeDaemonFactory(t, coll),
		InitialRetryDelay:    time.Millisecond,
		InitialRetryAttempts: -1,
	})
	daemon.SetLogger(logger.NewNopLogger())

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	count, err := daemon.Run(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run() error = %v, want DeadlineExceeded", err)
	}
	if count != 0 {
		t.Errorf("expected 0 successful collections, got %d", count)
	}
	if atomic.LoadInt32(&calls) != 1 {
		t.Errorf("expected exactly 1 collect attempt with retries disabled, got %d", calls)
	}
}
