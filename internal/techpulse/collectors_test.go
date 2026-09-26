// Package techpulse unit tests for collector registry construction.
package techpulse

import (
	"context"
	"testing"
	"time"

	"github.com/majiayu000/techpulse/internal/collector"
	"github.com/majiayu000/techpulse/internal/httpclient"
)

func TestBuildRegistryRegistersCollectors(t *testing.T) {
	reg := buildRegistry(Config{Timeout: 60})
	if got := len(reg.Names()); got != 7 {
		t.Fatalf("expected 7 registered sources, got %d", got)
	}
}

func TestRequestClientTimeout(t *testing.T) {
	if got := requestClientTimeout(90); got != 90*time.Second {
		t.Errorf("requestClientTimeout(90) = %v, want 90s", got)
	}
	if got := requestClientTimeout(0); got != 30*time.Second {
		t.Errorf("requestClientTimeout(0) = %v, want default 30s", got)
	}
}

func TestRequestHTTPOptions(t *testing.T) {
	if opts := requestHTTPOptions(0); opts != nil {
		t.Errorf("requestHTTPOptions(0) = %v, want nil", opts)
	}
	opts := requestHTTPOptions(45)
	if len(opts) != 1 {
		t.Fatalf("requestHTTPOptions(45) len = %d, want 1", len(opts))
	}
	c := httpclient.New(opts...)
	if c == nil {
		t.Fatal("httpclient.New with timeout option returned nil")
	}
}

func TestWithHTTPClientTimeout(t *testing.T) {
	c := &stubTimeoutCollector{timeout: 30 * time.Second}
	got := withHTTPClientTimeout(c, 75*time.Second)
	if got.Name() != "stub" {
		t.Errorf("Name() = %q, want stub", got.Name())
	}
	if c.timeout != 75*time.Second {
		t.Errorf("SetHTTPTimeout = %v, want 75s", c.timeout)
	}
}

type stubTimeoutCollector struct {
	timeout time.Duration
}

func (s *stubTimeoutCollector) Name() string { return "stub" }
func (s *stubTimeoutCollector) Validate() error {
	return nil
}
func (s *stubTimeoutCollector) Collect(_ context.Context, _ collector.Options) ([]collector.Article, error) {
	return nil, nil
}
func (s *stubTimeoutCollector) SetHTTPTimeout(d time.Duration) { s.timeout = d }
