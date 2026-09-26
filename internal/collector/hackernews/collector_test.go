package hackernews

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/majiayu000/techpulse/internal/collector"
)

func TestCollectorName(t *testing.T) {
	tests := []struct {
		category string
		expected string
	}{
		{"top", "hackernews_top"},
		{"new", "hackernews_new"},
		{"best", "hackernews_best"},
		{"ask", "hackernews_ask"},
		{"show", "hackernews_show"},
		{"job", "hackernews_job"},
	}

	for _, tt := range tests {
		c := New(tt.category)
		if c.Name() != tt.expected {
			t.Errorf("Name() = %s, want %s", c.Name(), tt.expected)
		}
	}
}

func TestCollectorValidate(t *testing.T) {
	// Valid categories
	for category := range ValidCategories {
		c := New(category)
		if err := c.Validate(); err != nil {
			t.Errorf("Validate() failed for valid category %s: %v", category, err)
		}
	}

	// Invalid category
	c := New("invalid")
	if err := c.Validate(); err == nil {
		t.Error("Validate() should fail for invalid category")
	}
}

func TestCollectorCollect(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/topstories.json":
			json.NewEncoder(w).Encode([]int{1, 2, 3})
		case "/item/1.json":
			json.NewEncoder(w).Encode(Item{
				ID:          1,
				Type:        "story",
				By:          "user1",
				Time:        time.Now().Unix(),
				Title:       "Test Story 1",
				URL:         "https://example.com/1",
				Score:       100,
				Descendants: 50,
			})
		case "/item/2.json":
			json.NewEncoder(w).Encode(Item{
				ID:          2,
				Type:        "story",
				By:          "user2",
				Time:        time.Now().Unix(),
				Title:       "Test Story 2",
				URL:         "https://example.com/2",
				Score:       200,
				Descendants: 30,
			})
		case "/item/3.json":
			// Deleted item
			json.NewEncoder(w).Encode(Item{
				ID:      3,
				Deleted: true,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newFastTestClient(server.URL)
	c := NewWithClient("top", client)

	ctx := context.Background()
	articles, err := c.Collect(ctx, collector.Options{Limit: 10})
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	// Should have 2 articles (item 3 is deleted)
	if len(articles) != 2 {
		t.Errorf("expected 2 articles, got %d", len(articles))
	}

	// Verify article fields
	if articles[0].Title != "Test Story 1" {
		t.Errorf("expected title 'Test Story 1', got '%s'", articles[0].Title)
	}
	if articles[0].Score != 100 {
		t.Errorf("expected score 100, got %d", articles[0].Score)
	}
	if articles[0].Source != "hackernews" {
		t.Errorf("expected source 'hackernews', got '%s'", articles[0].Source)
	}
}

func TestCollectorCollectAllItemsFail(t *testing.T) {
	// Story list succeeds, but every item fetch returns Firebase's "null"
	// body: Collect must surface an error instead of empty-but-success.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/topstories.json" {
			json.NewEncoder(w).Encode([]int{1, 2})
			return
		}
		w.Write([]byte("null"))
	}))
	defer server.Close()

	c := NewWithClient("top", newFastTestClient(server.URL))

	ctx := context.Background()
	articles, err := c.Collect(ctx, collector.Options{Limit: 10})
	if err == nil {
		t.Fatal("expected an error when every item fetch fails")
	}
	if articles != nil {
		t.Errorf("expected no articles on failure, got %d", len(articles))
	}
}

func TestCollectorCollectWithLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/topstories.json" {
			ids := make([]int, 100)
			for i := range ids {
				ids[i] = i + 1
			}
			json.NewEncoder(w).Encode(ids)
			return
		}
		// Parse ID from URL path: /item/123.json
		id := parseItemIDFromPath(r.URL.Path)
		json.NewEncoder(w).Encode(Item{
			ID:    id,
			Type:  "story",
			Time:  time.Now().Unix(),
			Title: "Test Story",
		})
	}))
	defer server.Close()

	client := newFastTestClient(server.URL)
	c := NewWithClient("top", client)

	ctx := context.Background()
	articles, err := c.Collect(ctx, collector.Options{Limit: 5})
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	if len(articles) != 5 {
		t.Errorf("expected 5 articles (limit), got %d", len(articles))
	}
}

func TestCollectorCollectWithTimeFilter(t *testing.T) {
	now := time.Now()
	yesterday := now.Add(-24 * time.Hour)
	twoDaysAgo := now.Add(-48 * time.Hour)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/topstories.json":
			json.NewEncoder(w).Encode([]int{1, 2})
		case "/item/1.json":
			json.NewEncoder(w).Encode(Item{
				ID:    1,
				Type:  "story",
				Time:  now.Unix(), // Recent
				Title: "Recent Story",
			})
		case "/item/2.json":
			json.NewEncoder(w).Encode(Item{
				ID:    2,
				Type:  "story",
				Time:  twoDaysAgo.Unix(), // Old
				Title: "Old Story",
			})
		}
	}))
	defer server.Close()

	client := newFastTestClient(server.URL)
	c := NewWithClient("top", client)

	ctx := context.Background()
	articles, err := c.Collect(ctx, collector.Options{
		Limit: 10,
		Since: yesterday, // Only get articles from the last 24 hours
	})
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	// Should only have the recent article
	if len(articles) != 1 {
		t.Errorf("expected 1 article (time filtered), got %d", len(articles))
	}
	if len(articles) > 0 && articles[0].Title != "Recent Story" {
		t.Errorf("expected 'Recent Story', got '%s'", articles[0].Title)
	}
}

func TestCollectorCollectSkipsDeadItems(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/topstories.json":
			json.NewEncoder(w).Encode([]int{1, 2})
		case "/item/1.json":
			json.NewEncoder(w).Encode(Item{
				ID:    1,
				Type:  "story",
				Time:  time.Now().Unix(),
				Title: "Live Story",
			})
		case "/item/2.json":
			json.NewEncoder(w).Encode(Item{
				ID:    2,
				Type:  "story",
				Time:  time.Now().Unix(),
				Title: "Dead Story",
				Dead:  true,
			})
		}
	}))
	defer server.Close()

	client := newFastTestClient(server.URL)
	c := NewWithClient("top", client)

	ctx := context.Background()
	articles, _ := c.Collect(ctx, collector.Options{Limit: 10})

	if len(articles) != 1 {
		t.Errorf("expected 1 article (dead filtered), got %d", len(articles))
	}
}

func TestCollectorToArticleWithoutURL(t *testing.T) {
	c := New("ask")
	item := &Item{
		ID:    12345,
		Type:  "story",
		By:    "author",
		Time:  time.Now().Unix(),
		Title: "Ask HN: Something",
		Text:  "Question content here",
	}

	article := c.toArticle(item)

	// Should generate HN discussion URL
	expectedURL := "https://news.ycombinator.com/item?id=12345"
	if article.URL != expectedURL {
		t.Errorf("expected URL '%s', got '%s'", expectedURL, article.URL)
	}
}

func TestCollectorToArticleMetadata(t *testing.T) {
	c := New("show")
	item := &Item{
		ID:   1,
		Type: "story",
		Time: time.Now().Unix(),
	}

	article := c.toArticle(item)

	if article.Metadata["category"] != "show" {
		t.Errorf("expected category 'show', got '%s'", article.Metadata["category"])
	}
	if article.Metadata["type"] != "story" {
		t.Errorf("expected type 'story', got '%s'", article.Metadata["type"])
	}
}
