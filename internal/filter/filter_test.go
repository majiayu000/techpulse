package filter

import (
	"testing"
	"time"

	"github.com/majiayu000/techpulse/internal/collector"
)

func TestToFiltered(t *testing.T) {
	articles := []collector.Article{
		{ID: "1", Title: "Test Article 1"},
		{ID: "2", Title: "Test Article 2"},
	}

	filtered := ToFiltered(articles)

	if len(filtered) != 2 {
		t.Errorf("expected 2 filtered articles, got %d", len(filtered))
	}

	if filtered[0].ID != "1" {
		t.Errorf("expected ID '1', got '%s'", filtered[0].ID)
	}

	if filtered[1].Title != "Test Article 2" {
		t.Errorf("expected title 'Test Article 2', got '%s'", filtered[1].Title)
	}
}

func TestToArticles(t *testing.T) {
	filtered := []FilteredArticle{
		{Article: collector.Article{ID: "1", Title: "Test 1"}, MatchedKeywords: []string{"AI"}},
		{Article: collector.Article{ID: "2", Title: "Test 2"}, IsDuplicate: true},
	}

	articles := ToArticles(filtered)

	if len(articles) != 2 {
		t.Errorf("expected 2 articles, got %d", len(articles))
	}

	if articles[0].ID != "1" {
		t.Errorf("expected ID '1', got '%s'", articles[0].ID)
	}
}

func TestToFilteredEmpty(t *testing.T) {
	filtered := ToFiltered(nil)

	if len(filtered) != 0 {
		t.Errorf("expected 0 filtered articles, got %d", len(filtered))
	}
}

func createTestArticle(id, title, content string) collector.Article {
	return collector.Article{
		ID:          id,
		Title:       title,
		Content:     content,
		Source:      "test",
		PublishedAt: time.Now(),
		CollectedAt: time.Now(),
	}
}
