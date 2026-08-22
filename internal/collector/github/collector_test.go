package github

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/majiayu000/techpulse/internal/collector"
)

func TestCollector_Name(t *testing.T) {
	tests := []struct {
		period   string
		language string
		expected string
	}{
		{"daily", "", "github_trending_daily"},
		{"weekly", "", "github_trending_weekly"},
		{"daily", "go", "github_trending_daily_go"},
		{"weekly", "rust", "github_trending_weekly_rust"},
	}

	for _, tt := range tests {
		c := NewWithOptions(tt.period, tt.language)
		if got := c.Name(); got != tt.expected {
			t.Errorf("Name() = %q, want %q", got, tt.expected)
		}
	}
}

func TestCollector_Validate(t *testing.T) {
	tests := []struct {
		period  string
		wantErr bool
	}{
		{"daily", false},
		{"weekly", false},
		{"monthly", false},
		{"yearly", true},
		{"", true},
	}

	for _, tt := range tests {
		c := NewWithOptions(tt.period, "")
		err := c.Validate()
		if (err != nil) != tt.wantErr {
			t.Errorf("Validate() period=%q error=%v, wantErr=%v", tt.period, err, tt.wantErr)
		}
	}
}

func TestCollector_Collect(t *testing.T) {
	html := `
	<article class="Box-row">
		<h2>
			<a href="/anthropics/claude-code" data-view-component="true">
				<span>anthropics</span> / <span>claude-code</span>
			</a>
		</h2>
		<p class="col-9 color-fg-muted my-1 pr-4">AI-powered coding assistant</p>
		<span itemprop="programmingLanguage">Go</span>
		<a href="/anthropics/claude-code/stargazers">1,234</a>
		<a href="/anthropics/claude-code/forks">56</a>
		<span>789 stars today</span>
	</article>
	<article class="Box-row">
		<h2>
			<a href="/rust-lang/rust" data-view-component="true">
				<span>rust-lang</span> / <span>rust</span>
			</a>
		</h2>
		<p class="col-9 color-fg-muted my-1 pr-4">Systems programming language</p>
		<span itemprop="programmingLanguage">Rust</span>
		<a href="/rust-lang/rust/stargazers">98,765</a>
		<a href="/rust-lang/rust/forks">4,321</a>
		<span>123 stars today</span>
	</article>
	`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
	}))
	defer server.Close()

	c := NewWithBaseURL(server.URL)
	articles, err := c.Collect(context.Background(), collector.Options{Limit: 10})

	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}

	if len(articles) != 2 {
		t.Fatalf("Collect() got %d articles, want 2", len(articles))
	}

	// Check first article
	if articles[0].Source != "github" {
		t.Errorf("Source = %q, want %q", articles[0].Source, "github")
	}
	if articles[0].URL != "https://github.com/anthropics/claude-code" {
		t.Errorf("URL = %q, want correct GitHub URL", articles[0].URL)
	}
}

func TestCollector_Collect_Limit(t *testing.T) {
	html := `
	<article class="Box-row">
		<h2><a href="/repo1/name"></a></h2>
	</article>
	<article class="Box-row">
		<h2><a href="/repo2/name"></a></h2>
	</article>
	<article class="Box-row">
		<h2><a href="/repo3/name"></a></h2>
	</article>
	`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(html))
	}))
	defer server.Close()

	c := NewWithBaseURL(server.URL)
	articles, err := c.Collect(context.Background(), collector.Options{Limit: 2})

	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}

	if len(articles) != 2 {
		t.Errorf("Collect() got %d articles, want 2 (limit)", len(articles))
	}
}

func TestCollector_Collect_MarkupChanged(t *testing.T) {
	// A substantial page (>10KB) that parses to 0 repos means the trending
	// markup changed; Collect must fail loudly instead of returning empty
	// success.
	html := fmt.Sprintf(`<html><body>%s</body></html>`, strings.Repeat("x", 12*1024))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(html))
	}))
	defer server.Close()

	c := NewWithBaseURL(server.URL)
	articles, err := c.Collect(context.Background(), collector.Options{Limit: 10})

	if err == nil {
		t.Fatalf("Collect() expected error for large page with 0 parsed repos, got %d articles", len(articles))
	}
	if !strings.Contains(err.Error(), "0 repos parsed") {
		t.Errorf("Collect() error = %v, want it to mention \"0 repos parsed\"", err)
	}
	if !strings.Contains(err.Error(), "markup may have changed") {
		t.Errorf("Collect() error = %v, want it to mention \"markup may have changed\"", err)
	}
}

func TestCollector_Collect_SmallEmptyPage(t *testing.T) {
	// A small page (<10KB) with no repos is treated as legitimately empty,
	// not as a markup failure.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("<html><body>empty</body></html>"))
	}))
	defer server.Close()

	c := NewWithBaseURL(server.URL)
	articles, err := c.Collect(context.Background(), collector.Options{Limit: 10})

	if err != nil {
		t.Fatalf("Collect() error = %v, want nil for small empty page", err)
	}
	if len(articles) != 0 {
		t.Errorf("Collect() got %d articles, want 0", len(articles))
	}
}

func TestCollector_BuildURL(t *testing.T) {
	tests := []struct {
		period   string
		language string
		expected string
	}{
		{"daily", "", "https://github.com/trending?since=daily"},
		{"weekly", "", "https://github.com/trending?since=weekly"},
		{"daily", "go", "https://github.com/trending/go?since=daily"},
		{"weekly", "rust", "https://github.com/trending/rust?since=weekly"},
	}

	for _, tt := range tests {
		c := NewWithOptions(tt.period, tt.language)
		if got := c.buildURL(); got != tt.expected {
			t.Errorf("buildURL() = %q, want %q", got, tt.expected)
		}
	}
}
