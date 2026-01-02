// Package techpulse test utilities and mock servers.
package techpulse

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/anthropic/autonomous-runner/internal/collector/hackernews"
)

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
