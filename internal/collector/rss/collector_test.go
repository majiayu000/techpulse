package rss

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/anthropic/autonomous-runner/internal/collector"
)

const testRSSFeed = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Test Feed</title>
    <link>https://example.com</link>
    <description>A test RSS feed</description>
    <item>
      <title>Test Article 1</title>
      <link>https://example.com/article1</link>
      <description>First test article content</description>
      <pubDate>Mon, 01 Jan 2026 10:00:00 +0000</pubDate>
      <author>author1@example.com</author>
      <guid>article-1</guid>
      <category>Tech</category>
      <category>AI</category>
    </item>
    <item>
      <title>Test Article 2</title>
      <link>https://example.com/article2</link>
      <description>Second test article content</description>
      <pubDate>Mon, 01 Jan 2026 09:00:00 +0000</pubDate>
      <creator>Author Two</creator>
      <guid>article-2</guid>
    </item>
  </channel>
</rss>`

func TestCollectorName(t *testing.T) {
	c := New([]Source{{Name: "Test", URL: "http://test.com/feed"}})
	if c.Name() != "rss" {
		t.Errorf("Name() = %s, want 'rss'", c.Name())
	}
}

func TestCollectorValidate(t *testing.T) {
	// Valid config
	c := New([]Source{{Name: "Test", URL: "http://test.com/feed"}})
	if err := c.Validate(); err != nil {
		t.Errorf("Validate() failed for valid config: %v", err)
	}

	// Invalid config (no sources)
	c = New([]Source{})
	if err := c.Validate(); err == nil {
		t.Error("Validate() should fail for empty sources")
	}

	// Nil sources
	c = New(nil)
	if err := c.Validate(); err == nil {
		t.Error("Validate() should fail for nil sources")
	}
}

func TestNewWithDefaults(t *testing.T) {
	c := NewWithDefaults()

	if len(c.sources) != len(DefaultSources) {
		t.Errorf("expected %d sources, got %d", len(DefaultSources), len(c.sources))
	}
}

func TestCollectorCollect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(testRSSFeed))
	}))
	defer server.Close()

	c := New([]Source{{Name: "TestFeed", URL: server.URL}})

	ctx := context.Background()
	articles, err := c.Collect(ctx, collector.Options{Limit: 10})
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	if len(articles) != 2 {
		t.Fatalf("expected 2 articles, got %d", len(articles))
	}

	// Verify first article
	if articles[0].Title != "Test Article 1" {
		t.Errorf("expected title 'Test Article 1', got '%s'", articles[0].Title)
	}
	if articles[0].URL != "https://example.com/article1" {
		t.Errorf("unexpected URL: %s", articles[0].URL)
	}
	if articles[0].Source != "rss" {
		t.Errorf("expected source 'rss', got '%s'", articles[0].Source)
	}
	if len(articles[0].Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(articles[0].Tags))
	}
}

func TestCollectorCollectWithLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(testRSSFeed))
	}))
	defer server.Close()

	c := New([]Source{{Name: "TestFeed", URL: server.URL}})

	ctx := context.Background()
	articles, err := c.Collect(ctx, collector.Options{Limit: 1})
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	if len(articles) != 1 {
		t.Errorf("expected 1 article (limit), got %d", len(articles))
	}
}

func TestCollectorCollectWithTimeFilter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(testRSSFeed))
	}))
	defer server.Close()

	c := New([]Source{{Name: "TestFeed", URL: server.URL}})

	// Filter for articles after 9:30
	since, _ := time.Parse(time.RFC1123Z, "Mon, 01 Jan 2026 09:30:00 +0000")

	ctx := context.Background()
	articles, err := c.Collect(ctx, collector.Options{
		Limit: 10,
		Since: since,
	})
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	// Only Article 1 should pass (published at 10:00)
	if len(articles) != 1 {
		t.Errorf("expected 1 article (time filtered), got %d", len(articles))
	}
}

func TestCollectorCollectMultipleSources(t *testing.T) {
	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(testRSSFeed))
	}))
	defer server1.Close()

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(testRSSFeed))
	}))
	defer server2.Close()

	c := New([]Source{
		{Name: "Feed1", URL: server1.URL},
		{Name: "Feed2", URL: server2.URL},
	})

	ctx := context.Background()
	articles, err := c.Collect(ctx, collector.Options{Limit: 100})
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	// 2 articles from each feed = 4 total
	if len(articles) != 4 {
		t.Errorf("expected 4 articles, got %d", len(articles))
	}
}

func TestCollectorCollectSourceError(t *testing.T) {
	goodServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(testRSSFeed))
	}))
	defer goodServer.Close()

	badServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer badServer.Close()

	c := New([]Source{
		{Name: "Good", URL: goodServer.URL},
		{Name: "Bad", URL: badServer.URL},
	})

	ctx := context.Background()
	articles, err := c.Collect(ctx, collector.Options{Limit: 10})

	// Should not return error, but skip bad source
	if err != nil {
		t.Fatalf("Collect() should not error: %v", err)
	}

	// Should still get articles from good source
	if len(articles) != 2 {
		t.Errorf("expected 2 articles from good source, got %d", len(articles))
	}
}

func TestCollectorParseTime(t *testing.T) {
	c := New(nil)

	tests := []struct {
		input    string
		wantZero bool
	}{
		{"Mon, 01 Jan 2026 10:00:00 +0000", false}, // RFC1123Z
		{"Mon, 01 Jan 2026 10:00:00 GMT", false},   // RFC1123
		{"2026-01-01T10:00:00Z", false},            // ISO 8601
		{"", false},                                 // Empty returns now
		{"invalid", false},                          // Invalid returns now
	}

	for _, tt := range tests {
		got := c.parseTime(tt.input)
		if tt.wantZero && !got.IsZero() {
			t.Errorf("parseTime(%q) should be zero", tt.input)
		}
		if !tt.wantZero && got.IsZero() {
			t.Errorf("parseTime(%q) should not be zero", tt.input)
		}
	}
}

func TestCollectorToArticle(t *testing.T) {
	c := New(nil)
	source := Source{Name: "TestSource", URL: "http://test.com/feed"}
	pubTime := time.Now()

	item := Item{
		Title:       "Test Title",
		Link:        "http://test.com/article",
		Description: "Test description",
		Author:      "author@test.com",
		GUID:        "unique-id",
		Categories:  []string{"Cat1", "Cat2"},
	}

	article := c.toArticle(item, source, pubTime)

	if article.Title != "Test Title" {
		t.Errorf("expected title 'Test Title', got '%s'", article.Title)
	}
	if article.Source != "rss" {
		t.Errorf("expected source 'rss', got '%s'", article.Source)
	}
	if article.Metadata["feed_name"] != "TestSource" {
		t.Errorf("expected feed_name 'TestSource', got '%s'", article.Metadata["feed_name"])
	}
	if article.Author != "author@test.com" {
		t.Errorf("expected author 'author@test.com', got '%s'", article.Author)
	}
}

func TestCollectorToArticleCreatorFallback(t *testing.T) {
	c := New(nil)
	source := Source{Name: "Test", URL: "http://test.com"}

	item := Item{
		Title:   "Test",
		Link:    "http://test.com/1",
		Creator: "Creator Name", // dc:creator
	}

	article := c.toArticle(item, source, time.Now())

	if article.Author != "Creator Name" {
		t.Errorf("expected creator fallback, got '%s'", article.Author)
	}
}

func TestCollectorToArticleGUIDFallback(t *testing.T) {
	c := New(nil)
	source := Source{Name: "Test", URL: "http://test.com"}

	item := Item{
		Title: "Test",
		Link:  "http://test.com/article",
		// No GUID
	}

	article := c.toArticle(item, source, time.Now())

	// Should use link as GUID fallback
	if article.SourceID != "http://test.com/article" {
		t.Errorf("expected link as sourceID fallback, got '%s'", article.SourceID)
	}
}

func TestHashString(t *testing.T) {
	// Same input should produce same hash
	h1 := hashString("test-string")
	h2 := hashString("test-string")
	if h1 != h2 {
		t.Errorf("same input should produce same hash: %s != %s", h1, h2)
	}

	// Different inputs should produce different hashes
	h3 := hashString("different-string")
	if h1 == h3 {
		t.Error("different inputs should produce different hashes")
	}

	// Hash should be 8 characters (hex)
	if len(h1) != 8 {
		t.Errorf("expected 8-char hash, got %d chars", len(h1))
	}
}
