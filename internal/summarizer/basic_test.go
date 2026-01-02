package summarizer

import (
	"context"
	"testing"

	"github.com/anthropic/autonomous-runner/internal/collector"
	"github.com/anthropic/autonomous-runner/internal/filter"
)

func TestNewBasicSummarizer(t *testing.T) {
	s := NewBasicSummarizer()
	if s == nil {
		t.Error("expected non-nil summarizer")
	}
}

func TestEnrich_Empty(t *testing.T) {
	s := NewBasicSummarizer()
	result, err := s.Enrich(context.Background(), nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d", len(result))
	}
}

func TestEnrich_SingleArticle(t *testing.T) {
	s := NewBasicSummarizer()
	articles := []filter.FilteredArticle{
		{
			Article: collector.Article{
				ID:       "1",
				Title:    "AI News",
				Score:    100,
				Comments: 50,
			},
			MatchedKeywords: []string{"AI"},
		},
	}

	result, err := s.Enrich(context.Background(), articles)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}
	// Base 5.0 + score>50: 0.5 + 1 keyword: 0.5 = 6.0
	if result[0].Importance != 6.0 {
		t.Errorf("expected importance 6.0, got %f", result[0].Importance)
	}
}

func TestEnrich_Sorting(t *testing.T) {
	s := NewBasicSummarizer()
	articles := []filter.FilteredArticle{
		{Article: collector.Article{ID: "1", Score: 10}},
		{Article: collector.Article{ID: "2", Score: 600}},
		{Article: collector.Article{ID: "3", Score: 200}},
	}

	result, err := s.Enrich(context.Background(), articles)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result[0].ID != "2" {
		t.Errorf("expected ID '2' first, got %s", result[0].ID)
	}
	if result[1].ID != "3" {
		t.Errorf("expected ID '3' second, got %s", result[1].ID)
	}
	if result[2].ID != "1" {
		t.Errorf("expected ID '1' third, got %s", result[2].ID)
	}
}

func TestEnrich_MultipleArticles(t *testing.T) {
	s := NewBasicSummarizer()
	articles := []filter.FilteredArticle{
		{
			Article:         collector.Article{ID: "1", Score: 50},
			MatchedKeywords: []string{"AI"},
		},
		{
			Article:         collector.Article{ID: "2", Score: 100, Comments: 60},
			MatchedKeywords: []string{"GPT", "LLM"},
		},
		{
			Article: collector.Article{ID: "3", Score: 30},
		},
	}

	result, err := s.Enrich(context.Background(), articles)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(result) != 3 {
		t.Fatalf("expected 3 results, got %d", len(result))
	}

	// Verify sorted by importance
	for i := 0; i < len(result)-1; i++ {
		if result[i].Importance < result[i+1].Importance {
			t.Errorf("expected descending order, got %f < %f at index %d",
				result[i].Importance, result[i+1].Importance, i)
		}
	}
}

func TestEnrich_PreservesOriginalData(t *testing.T) {
	s := NewBasicSummarizer()
	articles := []filter.FilteredArticle{
		{
			Article: collector.Article{
				ID:     "test-id",
				Title:  "Test Title",
				URL:    "https://example.com",
				Source: "test-source",
			},
			MatchedKeywords: []string{"AI"},
		},
	}

	result, err := s.Enrich(context.Background(), articles)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result[0].ID != "test-id" {
		t.Errorf("expected ID 'test-id', got %s", result[0].ID)
	}
	if result[0].Title != "Test Title" {
		t.Errorf("expected title 'Test Title', got %s", result[0].Title)
	}
	if result[0].URL != "https://example.com" {
		t.Errorf("expected URL 'https://example.com', got %s", result[0].URL)
	}
	if result[0].Source != "test-source" {
		t.Errorf("expected source 'test-source', got %s", result[0].Source)
	}
	if len(result[0].MatchedKeywords) != 1 {
		t.Errorf("expected 1 keyword, got %d", len(result[0].MatchedKeywords))
	}
}
