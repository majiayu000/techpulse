package rss

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/majiayu000/techpulse/internal/collector"
	"github.com/majiayu000/techpulse/internal/httpclient"
	"github.com/majiayu000/techpulse/internal/logger"
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

// testAtomFeed mirrors the shape of The Verge's Atom feed
// (https://www.theverge.com/rss/index.xml): a root <feed> element whose
// entries carry id/link/published/updated/author/content/category.
const testAtomFeed = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>The Verge</title>
  <id>https://www.theverge.com/rss/index.xml</id>
  <updated>2026-01-01T11:00:00Z</updated>
  <link rel="self" href="https://www.theverge.com/rss/index.xml"/>
  <link rel="alternate" type="text/html" href="https://www.theverge.com/"/>
  <entry>
    <id>https://www.theverge.com/24300000/verge-article-one</id>
    <title>Verge Article One</title>
    <link rel="alternate" type="text/html" href="https://www.theverge.com/24300000/verge-article-one"/>
    <published>2026-01-01T10:00:00.5Z</published>
    <updated>2026-01-01T10:05:00Z</updated>
    <author>
      <name>Alice Reporter</name>
    </author>
    <content type="html">&lt;p&gt;Verge article one content&lt;/p&gt;</content>
    <category term="Tech"/>
    <category term="AI"/>
  </entry>
  <entry>
    <id>tag:theverge.com,2026:verge-article-two</id>
    <title>Verge Article Two</title>
    <link href="https://www.theverge.com/24300001/verge-article-two"/>
    <updated>2026-01-01T09:00:00Z</updated>
    <author><name>Bob Writer</name></author>
    <summary>Second Verge article summary</summary>
    <category label="Gadgets" term="gadgets"/>
  </entry>
  <entry>
    <id>tag:theverge.com,2026:verge-article-three</id>
    <title>Verge Article Three</title>
    <published>2026-01-01T08:00:00Z</published>
    <summary>No links at all</summary>
  </entry>
</feed>`

// testRSSFeedUnknownDate covers items whose publication date cannot be
// parsed alongside an item that is unambiguously older than any Since cut.
const testRSSFeedUnknownDate = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Unknown Date Feed</title>
    <item>
      <title>Unknown Date Article</title>
      <link>https://example.com/unknown</link>
      <description>Date cannot be parsed</description>
      <pubDate>not-a-date</pubDate>
      <guid>unknown-1</guid>
    </item>
    <item>
      <title>Old Article</title>
      <link>https://example.com/old</link>
      <description>Older than the Since cutoff</description>
      <pubDate>Mon, 01 Jan 2026 08:00:00 +0000</pubDate>
      <guid>old-1</guid>
    </item>
  </channel>
</rss>`

// newFastCollector builds a collector from one or more feed lists whose
// HTTP client uses tiny retry backoff values so tests exercise the real
// retry path without sleeping through the default 500ms-10s schedule.
func newFastCollector(feedSources ...[]Source) *Collector {
	var sources []Source
	for _, fs := range feedSources {
		sources = append(sources, fs...)
	}
	c := New(sources)
	cfg := httpclient.DefaultRetryConfig()
	cfg.InitialDelay = time.Millisecond
	cfg.MaxDelay = 5 * time.Millisecond
	c.httpClient = httpclient.New(httpclient.WithRetryConfig(cfg))
	return c
}

// mustParseTime parses a fixture timestamp or fails the test.
func mustParseTime(t *testing.T, layout, value string) time.Time {
	t.Helper()
	ts, err := time.Parse(layout, value)
	if err != nil {
		t.Fatalf("time.Parse(%q, %q) failed: %v", layout, value, err)
	}
	return ts
}

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

	c := newFastCollector([]Source{{Name: "TestFeed", URL: server.URL}})

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

// TestCollectorCollectAtom verifies Atom feeds decode into the same article
// shape as RSS, using a fixture shaped like the real Verge feed.
func TestCollectorCollectAtom(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml")
		w.Write([]byte(testAtomFeed))
	}))
	defer server.Close()

	c := newFastCollector([]Source{{Name: "TheVerge", URL: server.URL}})
	c.WithLogger(logger.NewNopLogger())

	ctx := context.Background()
	articles, err := c.Collect(ctx, collector.Options{Limit: 10})
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	if len(articles) != 3 {
		t.Fatalf("expected 3 articles from Atom feed, got %d", len(articles))
	}
}

func TestCollectorCollectWithLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(testRSSFeed))
	}))
	defer server.Close()

	c := newFastCollector([]Source{{Name: "TestFeed", URL: server.URL}})

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
		w.Write([]byte(testRSSFeed))
	}))
	defer server.Close()

	c := newFastCollector([]Source{{Name: "TestFeed", URL: server.URL}})

	// Filter for articles published after 09:30
	since := mustParseTime(t, time.RFC1123Z, "Mon, 01 Jan 2026 09:30:00 +0000")

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

	c := newFastCollector(
		[]Source{{Name: "Feed1", URL: server1.URL}},
		[]Source{{Name: "Feed2", URL: server2.URL}},
	)

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

// TestCollectorCollectSourceError verifies partial success stays non-fatal:
// a failing feed is skipped (and logged), healthy feeds still contribute.
func TestCollectorCollectSourceError(t *testing.T) {
	goodServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(testRSSFeed))
	}))
	defer goodServer.Close()

	badServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer badServer.Close()

	c := newFastCollector(
		[]Source{{Name: "Good", URL: goodServer.URL}},
		[]Source{{Name: "Bad", URL: badServer.URL}},
	).WithLogger(logger.NewNopLogger())

	ctx := context.Background()
	articles, err := c.Collect(ctx, collector.Options{Limit: 10})

	// Partial success is not an error.
	if err != nil {
		t.Fatalf("Collect() should not error on partial failure: %v", err)
	}

	// Should still get articles from the good source
	if len(articles) != 2 {
		t.Errorf("expected 2 articles from good source, got %d", len(articles))
	}
}

// TestCollectorCollectAllFeedsFail verifies that when every feed fails,
// Collect surfaces an error naming the failed sources instead of silently
// returning zero articles.
func TestCollectorCollectAllFeedsFail(t *testing.T) {
	badServer1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer badServer1.Close()

	badServer2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer badServer2.Close()

	c := newFastCollector(
		[]Source{{Name: "BadOne", URL: badServer1.URL}},
		[]Source{{Name: "BadTwo", URL: badServer2.URL}},
	).WithLogger(logger.NewNopLogger())

	ctx := context.Background()
	articles, err := c.Collect(ctx, collector.Options{Limit: 10})

	if err == nil {
		t.Fatal("Collect() should return an error when all feeds fail")
	}
	if articles != nil {
		t.Errorf("expected no articles when all feeds fail, got %d", len(articles))
	}
	for _, want := range []string{"all 2 RSS feeds failed", "BadOne", "BadTwo"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q should mention %q", err.Error(), want)
		}
	}
}

// TestCollectorCollectUnsupportedFeed verifies fail-closed behavior: a
// document that is neither RSS nor Atom produces an error, not zero
// articles.
func TestCollectorCollectUnsupportedFeed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("<html><body>not a feed</body></html>"))
	}))
	defer server.Close()

	c := newFastCollector([]Source{{Name: "NotAFeed", URL: server.URL}}).
		WithLogger(logger.NewNopLogger())

	ctx := context.Background()
	articles, err := c.Collect(ctx, collector.Options{Limit: 10})

	if err == nil {
		t.Fatal("Collect() should error on an unsupported feed format")
	}
	if articles != nil {
		t.Errorf("expected no articles, got %d", len(articles))
	}
	if !strings.Contains(err.Error(), "unsupported feed format") {
		t.Errorf("error %q should mention unsupported feed format", err.Error())
	}
}

// TestCollectorCollectLimitRoundRobin verifies the limit is applied
// round-robin across feeds so later feeds are not truncated away entirely.
func TestCollectorCollectLimitRoundRobin(t *testing.T) {
	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(testRSSFeed))
	}))
	defer server1.Close()

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(testRSSFeed))
	}))
	defer server2.Close()

	c := newFastCollector(
		[]Source{{Name: "Feed1", URL: server1.URL}},
		[]Source{{Name: "Feed2", URL: server2.URL}},
	).WithLogger(logger.NewNopLogger())

	ctx := context.Background()
	articles, err := c.Collect(ctx, collector.Options{Limit: 2})
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	if len(articles) != 2 {
		t.Fatalf("expected 2 articles (1 per feed), got %d", len(articles))
	}
	if got := articles[0].Metadata["feed_name"]; got != "Feed1" {
		t.Errorf("articles[0] feed_name = %q, want Feed1", got)
	}
	if got := articles[1].Metadata["feed_name"]; got != "Feed2" {
		t.Errorf("articles[1] feed_name = %q, want Feed2", got)
	}
}

// TestCollectorCollectTimeFilterKeepsUnknownDates verifies items with
// unparseable dates are kept (with zero PublishedAt) under a Since filter,
// while known older dates are still filtered out.
func TestCollectorCollectTimeFilterKeepsUnknownDates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(testRSSFeedUnknownDate))
	}))
	defer server.Close()

	c := newFastCollector([]Source{{Name: "TestFeed", URL: server.URL}}).
		WithLogger(logger.NewNopLogger())

	since := mustParseTime(t, time.RFC3339, "2026-01-01T09:30:00Z")

	ctx := context.Background()
	articles, err := c.Collect(ctx, collector.Options{Limit: 10, Since: since})
	if err != nil {
		t.Fatalf("Since filtering should not error, got: %v", err)
	}

	if len(articles) != 1 {
		t.Fatalf("expected 1 article (unknown date kept, old filtered), got %d", len(articles))
	}
	if articles[0].Title != "Unknown Date Article" {
		t.Errorf("expected unknown-date article kept, got %q", articles[0].Title)
	}
	if !articles[0].PublishedAt.IsZero() {
		t.Errorf("unknown-date article should keep zero PublishedAt, got %v", articles[0].PublishedAt)
	}
}

// TestApplyLimit unit-tests the round-robin truncation directly.
func TestApplyLimit(t *testing.T) {
	mk := func(names ...string) []collector.Article {
		out := make([]collector.Article, 0, len(names))
		for _, n := range names {
			out = append(out, collector.Article{Title: n})
		}
		return out
	}
	titles := func(articles []collector.Article) []string {
		out := make([]string, 0, len(articles))
		for _, a := range articles {
			out = append(out, a.Title)
		}
		return out
	}
	equal := func(got, want []string) bool {
		if len(got) != len(want) {
			return false
		}
		for i := range got {
			if got[i] != want[i] {
				return false
			}
		}
		return true
	}

	tests := []struct {
		name  string
		feeds [][]collector.Article
		limit int
		want  []string
	}{
		{
			name:  "no limit keeps feed order",
			feeds: [][]collector.Article{mk("a1", "a2"), mk("b1")},
			limit: 0,
			want:  []string{"a1", "a2", "b1"},
		},
		{
			name:  "limit at or above total is a no-op",
			feeds: [][]collector.Article{mk("a1"), mk("b1")},
			limit: 10,
			want:  []string{"a1", "b1"},
		},
		{
			name:  "truncation is round-robin across feeds",
			feeds: [][]collector.Article{mk("a1", "a2", "a3"), mk("b1", "b2", "b3"), mk("c1")},
			limit: 4,
			want:  []string{"a1", "b1", "c1", "a2"},
		},
		{
			name:  "uneven feeds still interleave",
			feeds: [][]collector.Article{mk("a1", "a2", "a3"), mk("b1")},
			limit: 3,
			want:  []string{"a1", "b1", "a2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := applyLimit(tt.feeds, tt.limit)
			if !equal(titles(got), tt.want) {
				t.Errorf("applyLimit() = %v, want %v", titles(got), tt.want)
			}
		})
	}
}

// TestCollectorParseTime verifies common feed layouts parse exactly,
// including RFC 3339 with fractional seconds, and that unparseable or empty
// values yield the zero time instead of a fabricated "now".
func TestCollectorParseTime(t *testing.T) {
	c := New(nil)

	tests := []struct {
		input    string
		want     time.Time
		wantZero bool
	}{
		{input: "Mon, 01 Jan 2026 10:00:00 +0000", want: mustParseTime(t, time.RFC1123Z, "Mon, 01 Jan 2026 10:00:00 +0000")},
		{input: "Mon, 01 Jan 2026 10:00:00 GMT", want: mustParseTime(t, time.RFC1123, "Mon, 01 Jan 2026 10:00:00 GMT")},
		{input: "2026-01-01T10:00:00Z", want: mustParseTime(t, time.RFC3339, "2026-01-01T10:00:00Z")},
		{input: "2026-01-01T10:00:00.123456789Z", want: mustParseTime(t, time.RFC3339Nano, "2026-01-01T10:00:00.123456789Z")},
		{input: "2026-01-01T12:00:00.25+02:00", want: mustParseTime(t, time.RFC3339Nano, "2026-01-01T12:00:00.25+02:00")},
		{input: "", wantZero: true},
		{input: "invalid", wantZero: true},
	}

	for _, tt := range tests {
		got := c.parseTime(tt.input)
		if tt.wantZero {
			if !got.IsZero() {
				t.Errorf("parseTime(%q) = %v, want zero time", tt.input, got)
			}
			continue
		}
		if got.IsZero() {
			t.Errorf("parseTime(%q) returned zero time, want %v", tt.input, tt.want)
			continue
		}
		if !got.Equal(tt.want) {
			t.Errorf("parseTime(%q) = %v, want %v", tt.input, got, tt.want)
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

// Atom XHTML text constructs nest markup under summary/content; a plain
// string field would decode as empty and drop body text used by filters.
const testAtomXHTMLFeed = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>XHTML Feed</title>
  <entry>
    <id>tag:example.com,2026:xhtml-1</id>
    <title type="xhtml"><div xmlns="http://www.w3.org/1999/xhtml"><em>XHTML Title</em></div></title>
    <link rel="alternate" href="https://example.com/xhtml-1"/>
    <summary type="xhtml">
      <div xmlns="http://www.w3.org/1999/xhtml"><p>Keyword artificial intelligence in summary</p></div>
    </summary>
    <content type="xhtml">
      <div xmlns="http://www.w3.org/1999/xhtml"><p>Full body with machine learning</p></div>
    </content>
  </entry>
  <entry>
    <id>tag:example.com,2026:html-1</id>
    <title>HTML Title</title>
    <link href="https://example.com/html-1"/>
    <content type="html">&lt;p&gt;Escaped HTML content stays intact&lt;/p&gt;</content>
  </entry>
</feed>`

func TestDecodeAtomXHTMLTextConstructs(t *testing.T) {
	items, err := decodeFeed([]byte(testAtomXHTMLFeed), "https://example.com/feed.xml")
	if err != nil {
		t.Fatalf("decodeFeed: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if items[0].Title != "XHTML Title" {
		t.Errorf("xhtml title = %q, want nested text", items[0].Title)
	}
	if !strings.Contains(items[0].Description, "artificial intelligence") {
		t.Errorf("xhtml summary empty/lost: %q", items[0].Description)
	}
	if items[1].Description != "Escaped HTML content stays intact" {
		t.Errorf("html content = %q, want plain text without tags", items[1].Description)
	}
}

// Relative Atom hrefs must resolve against the feed URL / xml:base so
// SafeLinkURL and summary fetch see absolute http(s) destinations.
const testAtomRelativeLinkFeed = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom" xml:base="https://feeds.example.com/base/">
  <title>Relative Link Feed</title>
  <entry>
    <id>tag:example.com,2026:rel-1</id>
    <title>Feed-base relative</title>
    <link rel="alternate" href="articles/one"/>
    <summary>one</summary>
  </entry>
  <entry xml:base="https://cdn.example.com/posts/">
    <id>tag:example.com,2026:rel-2</id>
    <title>Entry-base relative</title>
    <link href="../two.html"/>
    <summary>two</summary>
  </entry>
  <entry>
    <id>tag:example.com,2026:rel-3</id>
    <title>Retrieval-URL relative</title>
    <link href="/root/three"/>
    <summary>three</summary>
  </entry>
</feed>`

func TestDecodeAtomRelativeLinks(t *testing.T) {
	// Intentionally omit feed-level xml:base for the third case by decoding
	// a feed whose only absolute base is the retrieval URL — covered via
	// entry without xml:base when feed xml:base is also empty.
	items, err := decodeFeed([]byte(testAtomRelativeLinkFeed), "https://www.example.com/rss/index.xml")
	if err != nil {
		t.Fatalf("decodeFeed: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("got %d items, want 3", len(items))
	}
	if items[0].Link != "https://feeds.example.com/base/articles/one" {
		t.Errorf("feed xml:base resolve = %q", items[0].Link)
	}
	if items[1].Link != "https://cdn.example.com/two.html" {
		t.Errorf("entry xml:base resolve = %q", items[1].Link)
	}
	// Absolute-path relative against feed xml:base (still present on feed).
	if items[2].Link != "https://feeds.example.com/root/three" {
		t.Errorf("absolute-path against feed base = %q", items[2].Link)
	}
}

func TestResolveAtomHrefAgainstFeedURL(t *testing.T) {
	got := resolveAtomHref("posts/a", "", "", "", "https://blog.example.com/atom.xml")
	want := "https://blog.example.com/posts/a"
	if got != want {
		t.Errorf("resolveAtomHref = %q, want %q", got, want)
	}
	if got := resolveAtomHref("https://already.example/x", "https://ignored/"); got != "https://already.example/x" {
		t.Errorf("absolute href mutated: %q", got)
	}
}

// Relative inner xml:base must compose against outer absolute bases before
// resolving the href (XML Base), not be skipped as non-absolute.
func TestResolveAtomHrefNestedRelativeXMLBase(t *testing.T) {
	got := resolveAtomHref("1", "", "posts/", "https://example.com/base/", "")
	want := "https://example.com/base/posts/1"
	if got != want {
		t.Errorf("nested relative xml:base = %q, want %q", got, want)
	}

	got = resolveAtomHref("item", "nest/", "posts/", "https://example.com/base/", "https://ignored.example/feed.xml")
	want = "https://example.com/base/posts/nest/item"
	if got != want {
		t.Errorf("link+entry relative xml:base = %q, want %q", got, want)
	}
}

const testAtomNestedRelativeBaseFeed = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom" xml:base="https://example.com/base/">
  <title>Nested Relative Base</title>
  <entry xml:base="posts/">
    <id>tag:example.com,2026:nest-1</id>
    <title>Nested</title>
    <link rel="alternate" href="1"/>
    <summary>one</summary>
  </entry>
</feed>`

func TestDecodeAtomNestedRelativeXMLBase(t *testing.T) {
	items, err := decodeFeed([]byte(testAtomNestedRelativeBaseFeed), "https://www.example.com/rss/index.xml")
	if err != nil {
		t.Fatalf("decodeFeed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	want := "https://example.com/base/posts/1"
	if items[0].Link != want {
		t.Errorf("nested relative xml:base decode = %q, want %q", items[0].Link, want)
	}
}

func TestCollectorCollectAtomXHTMLAndRelative(t *testing.T) {
	const feed = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Mixed</title>
  <entry>
    <id>tag:example.com,2026:mix-1</id>
    <title>Mixed Entry</title>
    <link rel="alternate" href="story/1"/>
    <summary type="xhtml"><div xmlns="http://www.w3.org/1999/xhtml"><p>nested summary text</p></div></summary>
  </entry>
</feed>`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml")
		w.Write([]byte(feed))
	}))
	defer server.Close()

	c := newFastCollector([]Source{{Name: "Mixed", URL: server.URL + "/feeds/tech.xml"}})
	articles, err := c.Collect(context.Background(), collector.Options{})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if len(articles) != 1 {
		t.Fatalf("got %d articles", len(articles))
	}
	if !strings.Contains(articles[0].Content, "nested summary text") {
		t.Errorf("content missing xhtml text: %q", articles[0].Content)
	}
	wantURL := server.URL + "/feeds/story/1"
	if articles[0].URL != wantURL {
		t.Errorf("URL = %q, want %q", articles[0].URL, wantURL)
	}
}

// Relative Atom links after an HTTP redirect must resolve against the final
// response URL, not the originally configured source URL.
func TestCollectorResolvesRelativeLinksAgainstRedirectURL(t *testing.T) {
	const feed = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Redirected</title>
  <entry>
    <id>tag:example.com,2026:redir-1</id>
    <title type="html">&lt;b&gt;Redirected News&lt;/b&gt;</title>
    <link rel="alternate" href="articles/one"/>
    <summary>body</summary>
  </entry>
</feed>`

	final := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/cdn/feed.xml" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/atom+xml")
		w.Write([]byte(feed))
	}))
	defer final.Close()

	start := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, final.URL+"/cdn/feed.xml", http.StatusFound)
	}))
	defer start.Close()

	c := newFastCollector([]Source{{Name: "Redirected", URL: start.URL + "/old/feed.xml"}})
	articles, err := c.Collect(context.Background(), collector.Options{})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if len(articles) != 1 {
		t.Fatalf("got %d articles, want 1", len(articles))
	}
	if articles[0].Title != "Redirected News" {
		t.Errorf("html title = %q, want plain text", articles[0].Title)
	}
	wantURL := final.URL + "/cdn/articles/one"
	if articles[0].URL != wantURL {
		t.Errorf("URL = %q, want final-host relative resolve %q", articles[0].URL, wantURL)
	}
}

// Adjacent XHTML block elements must keep a word boundary so phrase filters
// like "machine learning" still match after plain-text normalization.
func TestDecodeAtomXHTMLPreservesInterElementSeparators(t *testing.T) {
	const feed = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Separators</title>
  <entry>
    <id>tag:example.com,2026:sep-1</id>
    <title>Sep</title>
    <link href="https://example.com/sep-1"/>
    <summary type="xhtml"><div xmlns="http://www.w3.org/1999/xhtml"><p>machine</p><p>learning</p></div></summary>
  </entry>
</feed>`
	items, err := decodeFeed([]byte(feed), "https://example.com/feed.xml")
	if err != nil {
		t.Fatalf("decodeFeed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].Description != "machine learning" {
		t.Errorf("xhtml separators = %q, want %q", items[0].Description, "machine learning")
	}
}

// Default Atom type="text" must escape entity-decoded angle brackets so raw
// HTML does not reach Markdown digests (SanitizeMarkdownText does not escape <>).
func TestDecodeAtomTextEscapesLiteralMarkup(t *testing.T) {
	const feed = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Text Escape</title>
  <entry>
    <id>tag:example.com,2026:text-1</id>
    <title type="text">&lt;img src=x onerror=alert(1)&gt; AI</title>
    <link href="https://example.com/text-1"/>
    <summary type="text">plain &lt;b&gt;bold&lt;/b&gt; text</summary>
  </entry>
</feed>`
	items, err := decodeFeed([]byte(feed), "https://example.com/feed.xml")
	if err != nil {
		t.Fatalf("decodeFeed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if strings.Contains(items[0].Title, "<img") {
		t.Errorf("text title still has raw HTML: %q", items[0].Title)
	}
	if !strings.Contains(items[0].Title, "&lt;img") || !strings.Contains(items[0].Title, "AI") {
		t.Errorf("text title = %q, want escaped markup plus AI", items[0].Title)
	}
	if strings.Contains(items[0].Description, "<b>") {
		t.Errorf("text summary still has raw HTML: %q", items[0].Description)
	}
	if items[0].Description != "plain &lt;b&gt;bold&lt;/b&gt; text" {
		t.Errorf("text summary = %q, want escaped literals", items[0].Description)
	}
}

// type=html UnescapeString after tag strip, and type=xhtml entity-decoded
// CharData, must not leave raw angle brackets in digest-bound plain text.
func TestDecodeAtomHTMLAndXHTMLEscapeAngleBrackets(t *testing.T) {
	const feed = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>HTML/XHTML Escape</title>
  <entry>
    <id>tag:example.com,2026:html-esc-1</id>
    <title type="html">&lt;p&gt;Hello &amp;lt;world&amp;gt;&lt;/p&gt;</title>
    <link href="https://example.com/html-esc-1"/>
    <summary type="html">&lt;div&gt;see &amp;lt;img src=x&amp;gt; here&lt;/div&gt;</summary>
  </entry>
  <entry>
    <id>tag:example.com,2026:xhtml-esc-1</id>
    <title type="xhtml"><div xmlns="http://www.w3.org/1999/xhtml"><em>safe</em></div></title>
    <link href="https://example.com/xhtml-esc-1"/>
    <summary type="xhtml"><div xmlns="http://www.w3.org/1999/xhtml"><p>see &lt;b&gt;bold&lt;/b&gt; text</p></div></summary>
  </entry>
</feed>`
	items, err := decodeFeed([]byte(feed), "https://example.com/feed.xml")
	if err != nil {
		t.Fatalf("decodeFeed: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if strings.ContainsAny(items[0].Title, "<>") {
		t.Errorf("html title still has raw brackets: %q", items[0].Title)
	}
	if items[0].Title != "Hello &lt;world&gt;" {
		t.Errorf("html title = %q, want escaped literals after strip+unescape", items[0].Title)
	}
	if strings.ContainsAny(items[0].Description, "<>") {
		t.Errorf("html summary still has raw brackets: %q", items[0].Description)
	}
	if !strings.Contains(items[0].Description, "&lt;img") {
		t.Errorf("html summary = %q, want escaped img literal", items[0].Description)
	}
	if strings.ContainsAny(items[1].Description, "<>") {
		t.Errorf("xhtml summary still has raw brackets: %q", items[1].Description)
	}
	if items[1].Description != "see &lt;b&gt;bold&lt;/b&gt; text" {
		t.Errorf("xhtml summary = %q, want escaped CharData literals", items[1].Description)
	}
}

// Atom allows feed-level <author>; entries without their own author inherit it.
func TestDecodeAtomInheritsFeedLevelAuthor(t *testing.T) {
	const feed = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Authors</title>
  <author><name>Feed Author</name></author>
  <entry>
    <id>tag:example.com,2026:auth-1</id>
    <title>No entry author</title>
    <link href="https://example.com/auth-1"/>
    <summary>body</summary>
  </entry>
  <entry>
    <id>tag:example.com,2026:auth-2</id>
    <title>Has entry author</title>
    <link href="https://example.com/auth-2"/>
    <author><name>Entry Author</name></author>
    <summary>body</summary>
  </entry>
</feed>`
	items, err := decodeFeed([]byte(feed), "https://example.com/feed.xml")
	if err != nil {
		t.Fatalf("decodeFeed: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if items[0].Creator != "Feed Author" {
		t.Errorf("inherited author = %q, want Feed Author", items[0].Creator)
	}
	if items[1].Creator != "Entry Author" {
		t.Errorf("entry author = %q, want Entry Author", items[1].Creator)
	}
}
