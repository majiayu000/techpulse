// Package techpulse unit tests for collector registry construction.
package techpulse

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/majiayu000/techpulse/internal/collector"
)

func TestCollectTimeoutWrapper(t *testing.T) {
	tests := []struct {
		name     string
		opts     collector.Options
		fallback time.Duration
		sleep    time.Duration
		wantErr  bool
	}{
		{
			name:    "opts timeout expires",
			opts:    collector.Options{Timeout: 20 * time.Millisecond},
			sleep:   300 * time.Millisecond,
			wantErr: true,
		},
		{
			name:     "configured fallback expires when opts unset",
			opts:     collector.Options{},
			fallback: 20 * time.Millisecond,
			sleep:    300 * time.Millisecond,
			wantErr:  true,
		},
		{
			name:    "fast collect within timeout succeeds",
			opts:    collector.Options{Timeout: 5 * time.Second},
			sleep:   time.Millisecond,
			wantErr: false,
		},
		{
			name:    "no timeout configured runs without deadline",
			opts:    collector.Options{},
			sleep:   time.Millisecond,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inner := &fakeCollector{name: "slow", fn: func(ctx context.Context, _ collector.Options) ([]collector.Article, error) {
				select {
				case <-time.After(tt.sleep):
					return []collector.Article{testArticle(1, "late")}, nil
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}}
			wrapped := withCollectTimeout(inner, tt.fallback)

			if wrapped.Name() != "slow" {
				t.Errorf("Name() = %q, want passthrough %q", wrapped.Name(), "slow")
			}
			if err := wrapped.Validate(); err != nil {
				t.Errorf("Validate() = %v, want passthrough nil", err)
			}

			articles, err := wrapped.Collect(context.Background(), tt.opts)
			if tt.wantErr {
				if err == nil {
					t.Fatal("Collect() error = nil, want timeout error")
				}
				if !strings.Contains(err.Error(), "exceeded timeout") {
					t.Errorf("error should name the exceeded timeout, got: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Collect() error = %v, want nil", err)
			}
			if len(articles) != 1 {
				t.Errorf("Collect() returned %d articles, want 1", len(articles))
			}
		})
	}
}

func TestCollectTimeoutWrapperPreservesInnerError(t *testing.T) {
	innerErr := errors.New("source exploded")
	inner := &fakeCollector{name: "boom", fn: func(_ context.Context, _ collector.Options) ([]collector.Article, error) {
		return nil, innerErr
	}}
	wrapped := withCollectTimeout(inner, time.Second)

	_, err := wrapped.Collect(context.Background(), collector.Options{Timeout: 5 * time.Second})
	if !errors.Is(err, innerErr) {
		t.Errorf("Collect() error = %v, want wrapped %v", err, innerErr)
	}
	if strings.Contains(err.Error(), "exceeded timeout") {
		t.Errorf("non-timeout failure should not be labeled as timeout, got: %v", err)
	}
}

func TestBuildRegistryRegistersWrappedCollectors(t *testing.T) {
	reg := buildRegistry(Config{})
	if got := len(reg.Names()); got != 7 {
		t.Fatalf("expected 7 registered sources, got %d", got)
	}
}
