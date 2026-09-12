// Package techpulse test utilities and mock servers.
package techpulse

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/majiayu000/techpulse/internal/collector"
	"github.com/majiayu000/techpulse/internal/collector/hackernews"
	"github.com/majiayu000/techpulse/internal/filter"
	"github.com/majiayu000/techpulse/internal/logger"
	"github.com/majiayu000/techpulse/internal/storage"
	"github.com/majiayu000/techpulse/internal/summarizer"
)

// fakeCollector is a scriptable Collector for orchestrator tests.
type fakeCollector struct {
	name string
	fn   func(ctx context.Context, opts collector.Options) ([]collector.Article, error)
}

func (f *fakeCollector) Name() string {
	if f.name == "" {
		return "fake"
	}
	return f.name
}

func (f *fakeCollector) Collect(ctx context.Context, opts collector.Options) ([]collector.Article, error) {
	return f.fn(ctx, opts)
}

func (f *fakeCollector) Validate() error { return nil }

// testArticle returns a minimal article for tests.
func testArticle(id int, title string) collector.Article {
	return collector.Article{
		ID:       fmt.Sprintf("test-%d", id),
		Source:   "fake",
		SourceID: fmt.Sprintf("%d", id),
		Title:    title,
		URL:      fmt.Sprintf("https://example.com/%d", id),
	}
}

// newOrchestratorTechPulse builds a TechPulse around reg that writes into dir,
// mirroring how daemon_test wires instances without going through
// NewWithOptions. Pass filter.NewPipeline() to keep every article.
func newOrchestratorTechPulse(t *testing.T, reg *collector.Registry, dir string, pipeline *filter.Pipeline) *TechPulse {
	t.Helper()
	storeCfg := storage.DefaultConfig()
	storeCfg.BaseDir = dir
	return &TechPulse{
		config:     Config{Limit: 10},
		registry:   reg,
		pipeline:   pipeline,
		summarizer: summarizer.NewBasicSummarizer(),
		storage:    storage.NewMarkdownStorage(storeCfg),
		storeCfg:   storeCfg,
		log:        logger.NewNopLogger(),
	}
}

// testRSSFeed for mock RSS server.
const testRSSFeed = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Test Feed</title>
    <link>https://example.com</link>
    <item>
      <title>AI Breakthrough in 2026</title>
      <link>https://example.com/ai-breakthrough</link>
      <description>Major AI advancement</description>
      <pubDate>Wed, 01 Jan 2026 10:00:00 +0000</pubDate>
    </item>
    <item>
      <title>New LLM Model Released</title>
      <link>https://example.com/llm-release</link>
      <description>Claude 5 released</description>
      <pubDate>Wed, 01 Jan 2026 09:00:00 +0000</pubDate>
    </item>
  </channel>
</rss>`

// createMockHNServer creates a mock Hacker News API server.
func createMockHNServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/topstories.json":
			json.NewEncoder(w).Encode([]int{1, 2, 3})
		case "/item/1.json":
			json.NewEncoder(w).Encode(hackernews.Item{
				ID: 1, Type: "story", By: "user1",
				Time: time.Now().Unix(), Title: "Go 2.0 Released",
				URL: "https://go.dev/2", Score: 500, Descendants: 200,
			})
		case "/item/2.json":
			json.NewEncoder(w).Encode(hackernews.Item{
				ID: 2, Type: "story", By: "user2",
				Time: time.Now().Unix(), Title: "Rust vs Go Performance",
				URL: "https://rust-lang.org/perf", Score: 300, Descendants: 150,
			})
		case "/item/3.json":
			json.NewEncoder(w).Encode(hackernews.Item{
				ID: 3, Type: "story", By: "user3",
				Time: time.Now().Unix(), Title: "Random Unrelated Story",
				URL: "https://other.com", Score: 50, Descendants: 10,
			})
		default:
			http.NotFound(w, r)
		}
	}))
}

// createEmptyHNServer creates a mock HN server that returns no stories.
func createEmptyHNServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/topstories.json" {
			json.NewEncoder(w).Encode([]int{})
		}
	}))
}

// createMockRSSServer creates a mock RSS feed server.
func createMockRSSServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(testRSSFeed))
	}))
}
