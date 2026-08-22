package lobsters

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/majiayu000/techpulse/internal/collector"
)

func TestCollector_Name(t *testing.T) {
	tests := []struct {
		name     string
		feedType FeedType
		tags     []string
		want     string
	}{
		{"default", FeedHottest, nil, "lobsters_hottest"},
		{"newest", FeedNewest, nil, "lobsters_newest"},
		{"with tags", FeedHottest, []string{"go", "rust"}, "lobsters_hottest_go_rust"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewWithOptions(tt.feedType, tt.tags)
			if got := c.Name(); got != tt.want {
				t.Errorf("Name() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCollector_Validate(t *testing.T) {
	tests := []struct {
		name     string
		feedType FeedType
		wantErr  bool
	}{
		{"hottest", FeedHottest, false},
		{"newest", FeedNewest, false},
		{"active", FeedActive, false},
		{"invalid", "invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewWithOptions(tt.feedType, nil)
			err := c.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCollector_Collect(t *testing.T) {
	now := time.Now()
	stories := []Story{
		{
			ShortID:       "abc123",
			Title:         "Test Story about Go",
			URL:           "https://example.com/go",
			CreatedAt:     now,
			Score:         25,
			CommentCount:  10,
			SubmitterUser: "testuser",
			Tags:          []string{"go", "programming"},
			ShortIDURL:    "https://lobste.rs/s/abc123",
			CommentsURL:   "https://lobste.rs/s/abc123/test_story",
		},
		{
			ShortID:          "def456",
			Title:            "Self Post",
			CreatedAt:        now,
			Score:            15,
			SubmitterUser:    "another_user",
			DescriptionPlain: "This is a self post content",
			Tags:             []string{"meta"},
			ShortIDURL:       "https://lobste.rs/s/def456",
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(stories)
	}))
	defer server.Close()

	c := NewWithBaseURL(server.URL)
	articles, err := c.Collect(context.Background(), collector.Options{Limit: 10})
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}

	if len(articles) != 2 {
		t.Fatalf("expected 2 articles, got %d", len(articles))
	}

	// Check first article
	a := articles[0]
	if a.ID != "lobsters_abc123" {
		t.Errorf("ID = %v, want lobsters_abc123", a.ID)
	}
	if a.Source != "lobsters" {
		t.Errorf("Source = %v, want lobsters", a.Source)
	}
	if a.Title != "Test Story about Go" {
		t.Errorf("Title = %v, want 'Test Story about Go'", a.Title)
	}
	if a.URL != "https://example.com/go" {
		t.Errorf("URL = %v, want external URL", a.URL)
	}
	if a.Score != 25 {
		t.Errorf("Score = %v, want 25", a.Score)
	}

	// Check self post uses short_id_url
	b := articles[1]
	if b.URL != "https://lobste.rs/s/def456" {
		t.Errorf("self post URL = %v, want short_id_url", b.URL)
	}
	if b.Content != "This is a self post content" {
		t.Errorf("Content = %v, want description_plain", b.Content)
	}
}

func TestCollector_TimeFilter(t *testing.T) {
	now := time.Now()
	oldTime := now.Add(-48 * time.Hour)

	stories := []Story{
		{ShortID: "new", Title: "New Story", CreatedAt: now},
		{ShortID: "old", Title: "Old Story", CreatedAt: oldTime},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(stories)
	}))
	defer server.Close()

	c := NewWithBaseURL(server.URL)
	articles, err := c.Collect(context.Background(), collector.Options{
		Since: now.Add(-24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}

	if len(articles) != 1 {
		t.Fatalf("expected 1 article after time filter, got %d", len(articles))
	}
	if articles[0].SourceID != "new" {
		t.Errorf("expected 'new' story, got %v", articles[0].SourceID)
	}
}

func TestCollector_Limit(t *testing.T) {
	now := time.Now()
	stories := make([]Story, 10)
	for i := 0; i < 10; i++ {
		stories[i] = Story{
			ShortID:   string(rune('a' + i)),
			Title:     "Story",
			CreatedAt: now,
		}
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(stories)
	}))
	defer server.Close()

	c := NewWithBaseURL(server.URL)
	articles, err := c.Collect(context.Background(), collector.Options{Limit: 5})
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}

	if len(articles) != 5 {
		t.Errorf("expected 5 articles with limit, got %d", len(articles))
	}
}

func TestCollector_TagFilter(t *testing.T) {
	now := time.Now()
	stories := []Story{
		{ShortID: "go", Title: "Go Story", CreatedAt: now, Tags: []string{"go"}},
		{ShortID: "rust", Title: "Rust Story", CreatedAt: now, Tags: []string{"rust"}},
		{ShortID: "js", Title: "JS Story", CreatedAt: now, Tags: []string{"javascript"}},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(stories)
	}))
	defer server.Close()

	c := &Collector{
		httpClient: &http.Client{},
		baseURL:    server.URL,
		feedType:   FeedHottest,
		tags:       []string{"go", "rust"},
	}

	articles, err := c.Collect(context.Background(), collector.Options{})
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}

	if len(articles) != 2 {
		t.Fatalf("expected 2 articles with tag filter, got %d", len(articles))
	}
}

func TestFeedTypes(t *testing.T) {
	if !ValidFeedTypes[FeedHottest] {
		t.Error("FeedHottest should be valid")
	}
	if !ValidFeedTypes[FeedNewest] {
		t.Error("FeedNewest should be valid")
	}
	if !ValidFeedTypes[FeedActive] {
		t.Error("FeedActive should be valid")
	}
	if ValidFeedTypes["invalid"] {
		t.Error("'invalid' should not be valid")
	}
}
