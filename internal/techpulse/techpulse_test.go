// Package techpulse unit tests.
package techpulse

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/majiayu000/techpulse/internal/collector"
	"github.com/majiayu000/techpulse/internal/filter"
	"github.com/majiayu000/techpulse/internal/storage"
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

func TestTimeoutDuration(t *testing.T) {
	tests := []struct {
		name    string
		seconds int
		want    time.Duration
	}{
		{"zero means no deadline", 0, 0},
		{"negative means no deadline", -5, 0},
		{"positive converts", 60, 60 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := timeoutDuration(tt.seconds); got != tt.want {
				t.Errorf("timeoutDuration(%d) = %v, want %v", tt.seconds, got, tt.want)
			}
		})
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
	if tp.storeCfg.BaseDir != "/tmp/test" {
		t.Errorf("storeCfg.BaseDir = %q, want /tmp/test", tp.storeCfg.BaseDir)
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

// emptyFilterPipeline keeps only articles containing "quantumunicorn", which
// no test article contains, so filtering removes everything.
func emptyFilterPipeline() *filter.Pipeline {
	return filter.NewPipeline(filter.NewKeywordFilter([]string{"quantumunicorn"}, nil))
}

func writeDigest(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write digest: %v", err)
	}
}

func TestRunKeepsPreviousDigestWhenFilterRemovesEverything(t *testing.T) {
	dir := t.TempDir()
	prev := "# previous good digest\n\nKeep me.\n"
	digest := filepath.Join(dir, storage.DefaultConfig().DigestFile)
	writeDigest(t, digest, prev)

	reg := collector.NewRegistry()
	reg.Register(&fakeCollector{name: "src", fn: func(_ context.Context, _ collector.Options) ([]collector.Article, error) {
		return []collector.Article{testArticle(1, "hello world")}, nil
	}})

	tp := newOrchestratorTechPulse(t, reg, dir, emptyFilterPipeline())

	err := tp.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v, want nil (empty-filter run is recorded, not fatal)", err)
	}

	got, err := os.ReadFile(digest)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != prev {
		t.Errorf("previous digest was overwritten:\n got %q\nwant %q", got, prev)
	}

	archive := filepath.Join(dir, storage.DefaultConfig().ArchiveDir, time.Now().Format("2006-01-02")+".md")
	if _, statErr := os.Stat(archive); !os.IsNotExist(statErr) {
		t.Errorf("today's archive %s should not exist after preserved run", archive)
	}
}

func TestRunWritesTruthfulEmptyReportWhenNoPreviousDigest(t *testing.T) {
	dir := t.TempDir()

	reg := collector.NewRegistry()
	reg.Register(&fakeCollector{name: "src", fn: func(_ context.Context, _ collector.Options) ([]collector.Article, error) {
		return []collector.Article{testArticle(1, "hello world")}, nil
	}})

	tp := newOrchestratorTechPulse(t, reg, dir, emptyFilterPipeline())

	if err := tp.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, storage.DefaultConfig().DigestFile))
	if err != nil {
		t.Fatalf("expected a fresh (truthfully empty) digest: %v", err)
	}
	if !strings.Contains(string(content), "**0**") {
		t.Errorf("fresh digest should report zero articles, got:\n%s", content)
	}
}

func TestRunInterruptedDoesNotTouchExistingDigest(t *testing.T) {
	dir := t.TempDir()
	prev := "# previous good digest\n\nKeep me.\n"
	digest := filepath.Join(dir, storage.DefaultConfig().DigestFile)
	writeDigest(t, digest, prev)

	reg := collector.NewRegistry()
	reg.Register(&fakeCollector{name: "src", fn: func(_ context.Context, _ collector.Options) ([]collector.Article, error) {
		return []collector.Article{testArticle(1, "hello world")}, nil
	}})

	tp := newOrchestratorTechPulse(t, reg, dir, filter.NewPipeline())

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // simulate Ctrl+C before the run finishes

	err := tp.Run(ctx)
	if err == nil {
		t.Fatal("Run() error = nil, want interrupted error")
	}
	if !strings.Contains(err.Error(), "interrupted") {
		t.Errorf("error should mention interruption, got: %v", err)
	}
	if !strings.Contains(err.Error(), context.Canceled.Error()) {
		t.Errorf("error should wrap the context error, got: %v", err)
	}

	got, readErr := os.ReadFile(digest)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != prev {
		t.Errorf("digest was modified by interrupted run:\n got %q\nwant %q", got, prev)
	}
}

func TestHasPreviousDigest(t *testing.T) {
	dir := t.TempDir()
	reg := collector.NewRegistry()
	tp := newOrchestratorTechPulse(t, reg, dir, filter.NewPipeline())

	tests := []struct {
		name     string
		setup    func()
		wantHave bool
	}{
		{name: "missing file", setup: func() {}, wantHave: false},
		{
			name: "non-empty file",
			setup: func() {
				writeDigest(t, tp.digestPath(), "# digest")
			},
			wantHave: true,
		},
		{
			name: "empty file is not last-good output",
			setup: func() {
				writeDigest(t, tp.digestPath(), "")
			},
			wantHave: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			if got := tp.hasPreviousDigest(); got != tt.wantHave {
				t.Errorf("hasPreviousDigest() = %v, want %v", got, tt.wantHave)
			}
		})
	}
}
