package reddit

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/majiayu000/techpulse/internal/collector"
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

func TestCollector_AllSubredditsFail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer server.Close()

	c := NewWithBaseURL(server.URL, []Subreddit{{Name: "one"}, {Name: "two"}})

	articles, err := c.Collect(context.Background(), collector.Options{Limit: 10})
	if err == nil {
		t.Fatal("expected error when all subreddits fail, got nil")
	}
	if len(articles) != 0 {
		t.Errorf("expected no articles when all subreddits fail, got %d", len(articles))
	}
	for _, name := range []string{"one", "two"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error should name failed subreddit %q, got: %v", name, err)
		}
	}
}

func TestCollector_PartialFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/bad/") {
			http.Error(w, "server error", http.StatusInternalServerError)
			return
		}
		response := ListingResponse{}
		response.Data.Children = append(response.Data.Children, struct {
			Data Post `json:"data"`
		}{Data: Post{ID: "goodpost", Title: "Good Post", Created: float64(time.Now().Unix()), Subreddit: "good"}})
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	c := NewWithBaseURL(server.URL, []Subreddit{{Name: "good"}, {Name: "bad"}})

	articles, err := c.Collect(context.Background(), collector.Options{Limit: 10})
	if err != nil {
		t.Fatalf("partial failure should be non-fatal, got error: %v", err)
	}
	if len(articles) != 1 {
		t.Fatalf("expected 1 article from healthy subreddit, got %d", len(articles))
	}
	if articles[0].Metadata["subreddit"] != "good" {
		t.Errorf("expected article from 'good' subreddit, got %q", articles[0].Metadata["subreddit"])
	}
}

func TestCollector_LimitInterleavesSubreddits(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		sub := parts[1]
		response := ListingResponse{}
		for i := 0; i < 4; i++ {
			response.Data.Children = append(response.Data.Children, struct {
				Data Post `json:"data"`
			}{Data: Post{
				ID:      fmt.Sprintf("%s%d", sub, i),
				Title:   fmt.Sprintf("%s post %d", sub, i),
				Created: float64(time.Now().Unix()),
			}})
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	subs := []Subreddit{{Name: "aaa"}, {Name: "bbb"}, {Name: "ccc"}}
	c := NewWithBaseURL(server.URL, subs)

	articles, err := c.Collect(context.Background(), collector.Options{Limit: 6})
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}

	wantIDs := []string{"aaa0", "bbb0", "ccc0", "aaa1", "bbb1", "ccc1"}
	if len(articles) != len(wantIDs) {
		t.Fatalf("expected %d articles, got %d", len(wantIDs), len(articles))
	}
	for i, want := range wantIDs {
		if articles[i].SourceID != want {
			t.Errorf("articles[%d].SourceID = %q, want %q", i, articles[i].SourceID, want)
		}
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
		{"你好世界", 3, "你好世..."},                                          // multi-byte runes truncated at rune boundary
		{"日本語テスト", 10, "日本語テスト"},                                       // byte length exceeds maxLen but rune count does not: unchanged
		{strings.Repeat("é", 15), 10, strings.Repeat("é", 10) + "..."}, // 2-byte runes split mid-character before fix
	}

	for _, tt := range tests {
		got := truncateContent(tt.input, tt.maxLen)
		if got != tt.want {
			t.Errorf("truncateContent(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
		}
		if !utf8.ValidString(got) {
			t.Errorf("truncateContent(%q, %d) = %q, which is not valid UTF-8", tt.input, tt.maxLen, got)
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
