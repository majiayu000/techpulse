// Package techpulse unit tests.
package techpulse

import (
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Limit != 30 {
		t.Errorf("Limit = %d, want 30", cfg.Limit)
	}
	if cfg.Output != ".techpulse" {
		t.Errorf("Output = %s, want .techpulse", cfg.Output)
	}
	if cfg.Timeout != 60 {
		t.Errorf("Timeout = %d, want 60", cfg.Timeout)
	}
}

func TestNew(t *testing.T) {
	tp := New()
	if tp == nil {
		t.Fatal("New() returned nil")
	}
	sources := tp.AvailableSources()
	if len(sources) != 7 {
		t.Errorf("expected 7 sources (hackernews top/ask/show, rss, github, reddit, lobsters), got %d", len(sources))
	}
}

func TestNewWithOptions(t *testing.T) {
	cfg := Config{
		Limit:   10,
		Sources: []string{"hackernews_top"},
		Output:  "/tmp/test",
		Timeout: 30,
	}
	tp := NewWithOptions(cfg)
	if tp.config.Limit != 10 {
		t.Errorf("Limit = %d, want 10", tp.config.Limit)
	}
}

func TestAvailableSources(t *testing.T) {
	tp := New()
	sources := tp.AvailableSources()

	expected := map[string]bool{
		"hackernews_top":        false,
		"hackernews_ask":        false,
		"hackernews_show":       false,
		"rss":                   false,
		"github_trending_daily": false,
		"reddit":                false,
		"lobsters_hottest":      false,
	}
	for _, s := range sources {
		if _, ok := expected[s]; ok {
			expected[s] = true
		}
	}
	for name, found := range expected {
		if !found {
			t.Errorf("missing %s source", name)
		}
	}
}
