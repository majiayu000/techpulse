package reddit

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/anthropic/autonomous-runner/internal/collector"
)

func TestCollector_Name(t *testing.T) {
	c := New()
	if c.Name() != "reddit" {
		t.Errorf("expected name 'reddit', got %s", c.Name())
	}
}

func TestCollector_Validate(t *testing.T) {
	tests := []struct {
		name    string
		c       *Collector
		wantErr bool
	}{
		{
			name:    "valid default",
			c:       New(),
			wantErr: false,
		},
		{
			name:    "no subreddits",
			c:       NewWithSubreddits(nil),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.c.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCollector_Collect(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := ListingResponse{}
		response.Data.Children = []struct {
			Data Post `json:"data"`
		}{
			{Data: Post{
				ID:          "abc123",
				Title:       "Test Post about AI",
				URL:         "https://example.com/ai",
				Author:      "testuser",
				Score:       100,
				NumComments: 50,
				Created:     float64(time.Now().Unix()),
				Subreddit:   "MachineLearning",
			}},
			{Data: Post{
				ID:        "def456",
				Title:     "Self Post",
				Selftext:  "This is a self post",
				Author:    "another_user",
				Score:     25,
				Created:   float64(time.Now().Unix()),
				Subreddit: "MachineLearning",
				IsSelf:    true,
				Permalink: "/r/MachineLearning/comments/def456/self_post/",
			}},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	subreddits := []Subreddit{{Name: "MachineLearning", Category: "ai"}}
	c := NewWithBaseURL(server.URL, subreddits)

	articles, err := c.Collect(context.Background(), collector.Options{Limit: 10})
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}

	if len(articles) != 2 {
		t.Fatalf("expected 2 articles, got %d", len(articles))
	}

	// Check first article
	a := articles[0]
	if a.ID != "reddit_abc123" {
		t.Errorf("expected ID 'reddit_abc123', got %s", a.ID)
	}
	if a.Source != "reddit" {
		t.Errorf("expected source 'reddit', got %s", a.Source)
	}
	if a.Title != "Test Post about AI" {
		t.Errorf("expected title 'Test Post about AI', got %s", a.Title)
	}
	if a.URL != "https://example.com/ai" {
		t.Errorf("expected URL 'https://example.com/ai', got %s", a.URL)
	}

	// Check self post uses permalink
	b := articles[1]
	if b.URL != "https://www.reddit.com/r/MachineLearning/comments/def456/self_post/" {
		t.Errorf("self post should use permalink, got %s", b.URL)
	}
}

func TestCollector_TimeFilter(t *testing.T) {
	now := time.Now()
	oldTime := now.Add(-48 * time.Hour)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := ListingResponse{}
		response.Data.Children = []struct {
			Data Post `json:"data"`
		}{
			{Data: Post{ID: "new", Title: "New Post", Created: float64(now.Unix())}},
			{Data: Post{ID: "old", Title: "Old Post", Created: float64(oldTime.Unix())}},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	c := NewWithBaseURL(server.URL, []Subreddit{{Name: "test"}})

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
		t.Errorf("expected 'new' post, got %s", articles[0].SourceID)
	}
}

func TestCollector_Limit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := ListingResponse{}
		for i := 0; i < 10; i++ {
			response.Data.Children = append(response.Data.Children, struct {
				Data Post `json:"data"`
			}{Data: Post{ID: string(rune('a' + i)), Title: "Post", Created: float64(time.Now().Unix())}})
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	c := NewWithBaseURL(server.URL, []Subreddit{{Name: "test"}})

	articles, err := c.Collect(context.Background(), collector.Options{Limit: 5})
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}

	if len(articles) != 5 {
		t.Errorf("expected 5 articles with limit, got %d", len(articles))
	}
}

func TestPost_CreatedTime(t *testing.T) {
	p := Post{Created: 1704067200}
	expected := time.Unix(1704067200, 0)
	if !p.CreatedTime().Equal(expected) {
		t.Errorf("CreatedTime() = %v, want %v", p.CreatedTime(), expected)
	}
}

func TestTruncateContent(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"short", 10, "short"},
		{"this is a long string", 10, "this is a ..."},
		{"", 10, ""},
	}

	for _, tt := range tests {
		got := truncateContent(tt.input, tt.maxLen)
		if got != tt.want {
			t.Errorf("truncateContent(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
		}
	}
}

func TestBuildTags(t *testing.T) {
	p := Post{Subreddit: "MachineLearning", Flair: "Research"}
	sub := Subreddit{Name: "MachineLearning", Category: "ai"}

	tags := buildTags(p, sub)
	if len(tags) != 3 {
		t.Errorf("expected 3 tags, got %d", len(tags))
	}
}
