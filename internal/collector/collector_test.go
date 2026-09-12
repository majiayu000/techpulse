package collector

import (
	"testing"
	"time"
)

func TestArticleFields(t *testing.T) {
	now := time.Now()
	article := Article{
		ID:          "test-1",
		Source:      "test",
		SourceID:    "123",
		Title:       "Test Article",
		URL:         "https://example.com/article",
		Content:     "This is test content",
		Author:      "testuser",
		Score:       100,
		Comments:    50,
		Tags:        []string{"AI", "Go"},
		Metadata:    map[string]string{"key": "value"},
		PublishedAt: now,
		CollectedAt: now,
	}

	if article.ID != "test-1" {
		t.Errorf("expected ID 'test-1', got '%s'", article.ID)
	}
	if article.Source != "test" {
		t.Errorf("expected Source 'test', got '%s'", article.Source)
	}
	if article.Score != 100 {
		t.Errorf("expected Score 100, got %d", article.Score)
	}
	if len(article.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(article.Tags))
	}
	if article.Metadata["key"] != "value" {
		t.Errorf("expected metadata key='value', got '%s'", article.Metadata["key"])
	}
}

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()

	if opts.Limit != 30 {
		t.Errorf("expected Limit 30, got %d", opts.Limit)
	}
	if opts.Timeout != 0 {
		t.Errorf("expected Timeout 0 (no Collect-level deadline), got %v", opts.Timeout)
	}
	if !opts.Since.IsZero() {
		t.Errorf("expected Since to be zero, got %v", opts.Since)
	}
	if opts.Category != "" {
		t.Errorf("expected Category to be empty, got '%s'", opts.Category)
	}
}

func TestOptionsWithCustomValues(t *testing.T) {
	since := time.Now().Add(-24 * time.Hour)
	opts := Options{
		Limit:    10,
		Since:    since,
		Category: "tech",
		Timeout:  60 * time.Second,
	}

	if opts.Limit != 10 {
		t.Errorf("expected Limit 10, got %d", opts.Limit)
	}
	if !opts.Since.Equal(since) {
		t.Errorf("expected Since %v, got %v", since, opts.Since)
	}
	if opts.Category != "tech" {
		t.Errorf("expected Category 'tech', got '%s'", opts.Category)
	}
	if opts.Timeout != 60*time.Second {
		t.Errorf("expected Timeout 60s, got %v", opts.Timeout)
	}
}

func TestResultFields(t *testing.T) {
	now := time.Now()
	result := Result{
		Source: "test-collector",
		Articles: []Article{
			{ID: "1", Title: "Article 1"},
			{ID: "2", Title: "Article 2"},
		},
		Error:     nil,
		Duration:  5 * time.Second,
		Timestamp: now,
	}

	if result.Source != "test-collector" {
		t.Errorf("expected Source 'test-collector', got '%s'", result.Source)
	}
	if len(result.Articles) != 2 {
		t.Errorf("expected 2 articles, got %d", len(result.Articles))
	}
	if result.Error != nil {
		t.Errorf("expected nil Error, got %v", result.Error)
	}
	if result.Duration != 5*time.Second {
		t.Errorf("expected Duration 5s, got %v", result.Duration)
	}
}

func TestResultWithError(t *testing.T) {
	result := Result{
		Source:   "failed-collector",
		Articles: nil,
		Error:    ErrCollectionFailed,
	}

	if result.Error != ErrCollectionFailed {
		t.Errorf("expected ErrCollectionFailed, got %v", result.Error)
	}
	if result.Articles != nil {
		t.Errorf("expected nil Articles, got %v", result.Articles)
	}
}

// ErrCollectionFailed is a sentinel error for testing.
var ErrCollectionFailed = &collectionError{msg: "collection failed"}

type collectionError struct {
	msg string
}

func (e *collectionError) Error() string {
	return e.msg
}
