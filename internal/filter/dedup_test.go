package filter

import (
	"testing"

	"github.com/anthropic/autonomous-runner/internal/collector"
)

func TestDedupFilter_Name(t *testing.T) {
	f := NewDedupFilter(0.8)
	if f.Name() != "dedup" {
		t.Errorf("expected name 'dedup', got '%s'", f.Name())
	}
}

func TestDedupFilter_RemoveDuplicates(t *testing.T) {
	f := NewDedupFilter(0.8)

	articles := []collector.Article{
		createTestArticle("1", "Introduction to AI", ""),
		createTestArticle("2", "Introduction to AI", ""), // duplicate
		createTestArticle("3", "Different topic", ""),
	}

	result := f.Apply(articles)

	if len(result) != 2 {
		t.Errorf("expected 2 unique articles, got %d", len(result))
	}

	if result[0].ID != "1" || result[1].ID != "3" {
		t.Errorf("unexpected article IDs: %s, %s", result[0].ID, result[1].ID)
	}
}

func TestDedupFilter_NormalizesWhitespace(t *testing.T) {
	f := NewDedupFilter(0.8)

	articles := []collector.Article{
		createTestArticle("1", "Introduction to AI", ""),
		createTestArticle("2", "Introduction  to  AI", ""), // extra spaces
		createTestArticle("3", " Introduction to AI ", ""), // leading/trailing
	}

	result := f.Apply(articles)

	if len(result) != 1 {
		t.Errorf("expected 1 unique article after normalization, got %d", len(result))
	}
}

func TestDedupFilter_CaseInsensitive(t *testing.T) {
	f := NewDedupFilter(0.8)

	articles := []collector.Article{
		createTestArticle("1", "Introduction to AI", ""),
		createTestArticle("2", "INTRODUCTION TO AI", ""),
		createTestArticle("3", "introduction to ai", ""),
	}

	result := f.Apply(articles)

	if len(result) != 1 {
		t.Errorf("expected 1 unique article (case insensitive), got %d", len(result))
	}
}

func TestDedupFilter_Reset(t *testing.T) {
	f := NewDedupFilter(0.8)

	articles := []collector.Article{
		createTestArticle("1", "Test Article", ""),
	}

	result1 := f.Apply(articles)
	if len(result1) != 1 {
		t.Errorf("expected 1 article first time, got %d", len(result1))
	}

	// Without reset, same article would be duplicate
	result2 := f.Apply(articles)
	if len(result2) != 0 {
		t.Errorf("expected 0 articles (duplicate), got %d", len(result2))
	}

	// After reset, should work again
	f.Reset()
	result3 := f.Apply(articles)
	if len(result3) != 1 {
		t.Errorf("expected 1 article after reset, got %d", len(result3))
	}
}

func TestDedupFilter_ApplyToFiltered(t *testing.T) {
	f := NewDedupFilter(0.8)

	filtered := []FilteredArticle{
		{Article: createTestArticle("1", "AI Article", ""), MatchedKeywords: []string{"AI"}},
		{Article: createTestArticle("2", "AI Article", ""), MatchedKeywords: []string{"AI"}},
		{Article: createTestArticle("3", "Different", ""), MatchedKeywords: []string{"LLM"}},
	}

	result := f.ApplyToFiltered(filtered)

	if len(result) != 2 {
		t.Errorf("expected 2 unique articles, got %d", len(result))
	}

	// Keywords should be preserved
	if len(result[0].MatchedKeywords) != 1 {
		t.Errorf("expected 1 keyword preserved, got %d", len(result[0].MatchedKeywords))
	}
}

func TestDedupFilter_IgnoresPunctuation(t *testing.T) {
	f := NewDedupFilter(0.8)

	articles := []collector.Article{
		createTestArticle("1", "Hello, World!", ""),
		createTestArticle("2", "Hello World", ""),
	}

	result := f.Apply(articles)

	if len(result) != 1 {
		t.Errorf("expected 1 unique (punctuation ignored), got %d", len(result))
	}
}

func TestNewDedupFilter_DefaultSimilarity(t *testing.T) {
	// Invalid similarity should default to 0.8
	f := NewDedupFilter(0)
	if f.similarity != 0.8 {
		t.Errorf("expected default 0.8, got %f", f.similarity)
	}

	f = NewDedupFilter(-1)
	if f.similarity != 0.8 {
		t.Errorf("expected default 0.8, got %f", f.similarity)
	}

	f = NewDedupFilter(1.5)
	if f.similarity != 0.8 {
		t.Errorf("expected default 0.8, got %f", f.similarity)
	}
}
