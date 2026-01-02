package filter

import (
	"testing"

	"github.com/anthropic/autonomous-runner/internal/collector"
)

func TestPipeline_EmptyPipeline(t *testing.T) {
	p := NewPipeline()

	articles := []collector.Article{
		createTestArticle("1", "Test", ""),
		createTestArticle("2", "Test 2", ""),
	}

	result := p.Process(articles)

	if len(result) != 2 {
		t.Errorf("expected 2 articles (no filter), got %d", len(result))
	}
}

func TestPipeline_SingleFilter(t *testing.T) {
	include := []string{"AI"}
	p := NewPipeline(NewKeywordFilter(include, nil))

	articles := []collector.Article{
		createTestArticle("1", "AI Article", ""),
		createTestArticle("2", "Cooking Recipe", ""),
	}

	result := p.Process(articles)

	if len(result) != 1 {
		t.Errorf("expected 1 matched article, got %d", len(result))
	}
}

func TestPipeline_WithDedup(t *testing.T) {
	include := []string{"AI"}
	p := NewPipeline(NewKeywordFilter(include, nil)).WithDedup(0.8)

	articles := []collector.Article{
		createTestArticle("1", "AI Article", ""),
		createTestArticle("2", "AI Article", ""), // duplicate
		createTestArticle("3", "AI News", ""),
	}

	result := p.Process(articles)

	if len(result) != 2 {
		t.Errorf("expected 2 unique matched articles, got %d", len(result))
	}
}

func TestPipeline_Add(t *testing.T) {
	p := NewPipeline()
	p.Add(NewKeywordFilter([]string{"AI"}, nil))

	articles := []collector.Article{
		createTestArticle("1", "AI Article", ""),
		createTestArticle("2", "Other", ""),
	}

	result := p.Process(articles)

	if len(result) != 1 {
		t.Errorf("expected 1 matched after Add, got %d", len(result))
	}
}

func TestPipeline_ChainedAdd(t *testing.T) {
	p := NewPipeline().
		Add(NewKeywordFilter([]string{"AI"}, nil)).
		WithDedup(0.8)

	articles := []collector.Article{
		createTestArticle("1", "AI Article", ""),
	}

	result := p.Process(articles)

	if len(result) != 1 {
		t.Errorf("expected 1 article, got %d", len(result))
	}
}

func TestPipeline_Reset(t *testing.T) {
	p := NewPipeline().WithDedup(0.8)

	articles := []collector.Article{
		createTestArticle("1", "Test", ""),
	}

	result1 := p.Process(articles)
	if len(result1) != 1 {
		t.Errorf("expected 1 first time, got %d", len(result1))
	}

	// Same article would be duplicate
	result2 := p.Process(articles)
	if len(result2) != 0 {
		t.Errorf("expected 0 (duplicate), got %d", len(result2))
	}

	// Reset and try again
	p.Reset()
	result3 := p.Process(articles)
	if len(result3) != 1 {
		t.Errorf("expected 1 after reset, got %d", len(result3))
	}
}

func TestPipeline_OnlyDedup(t *testing.T) {
	p := NewPipeline().WithDedup(0.8)

	articles := []collector.Article{
		createTestArticle("1", "Test Article", ""),
		createTestArticle("2", "Test Article", ""), // duplicate
		createTestArticle("3", "Other Article", ""),
	}

	result := p.Process(articles)

	if len(result) != 2 {
		t.Errorf("expected 2 unique articles, got %d", len(result))
	}
}

func TestDefaultPipeline(t *testing.T) {
	p := DefaultPipeline()

	articles := []collector.Article{
		createTestArticle("1", "AI and LLM News", ""),
		createTestArticle("2", "Cooking Recipe", ""),
		createTestArticle("3", "GPT Tutorial", ""),
		createTestArticle("4", "GPT Tutorial", ""), // duplicate
	}

	result := p.Process(articles)

	if len(result) != 2 {
		t.Errorf("expected 2 (filtered + deduped), got %d", len(result))
	}

	// Check that matched keywords are present
	if len(result[0].MatchedKeywords) == 0 {
		t.Error("expected matched keywords to be populated")
	}
}

func TestPipeline_MultipleFilters(t *testing.T) {
	p := NewPipeline(
		NewKeywordFilter([]string{"AI"}, nil),
		NewKeywordFilter([]string{"LLM"}, nil),
	)

	articles := []collector.Article{
		createTestArticle("1", "AI only", ""),
		createTestArticle("2", "LLM only", ""),
		createTestArticle("3", "AI and LLM", ""),
		createTestArticle("4", "Other topic", ""),
	}

	result := p.Process(articles)

	// First filter keeps AI articles, second keeps LLM from those
	// So only "AI and LLM" passes both
	if len(result) != 1 {
		t.Errorf("expected 1 article passing both filters, got %d", len(result))
	}

	if result[0].ID != "3" {
		t.Errorf("expected ID '3', got '%s'", result[0].ID)
	}
}
