package summarizer

import (
	"testing"

	"github.com/anthropic/autonomous-runner/internal/collector"
	"github.com/anthropic/autonomous-runner/internal/filter"
)

func TestEnrichedArticle(t *testing.T) {
	article := EnrichedArticle{
		FilteredArticle: filter.FilteredArticle{
			Article: collector.Article{
				ID:     "test-1",
				Title:  "Test Article",
				URL:    "https://example.com",
				Source: "test",
			},
			MatchedKeywords: []string{"AI", "GPT"},
		},
		Importance: 8.5,
		Summary:    "Test summary",
		AITags:     []string{"technology", "ai"},
	}

	if article.Importance != 8.5 {
		t.Errorf("expected importance 8.5, got %f", article.Importance)
	}
	if article.Summary != "Test summary" {
		t.Errorf("expected summary 'Test summary', got %s", article.Summary)
	}
	if len(article.AITags) != 2 {
		t.Errorf("expected 2 AI tags, got %d", len(article.AITags))
	}
}

func TestReport(t *testing.T) {
	report := Report{
		Title: "Daily Digest",
		Date:  "2026-01-01",
		TopStories: []EnrichedArticle{
			{
				FilteredArticle: filter.FilteredArticle{
					Article: collector.Article{Title: "Story 1"},
				},
				Importance: 9.0,
			},
		},
		Stats: Stats{
			TotalArticles: 10,
			BySource:      map[string]int{"hackernews": 5, "rss": 5},
			AvgImportance: 7.5,
		},
		Content: "# Report",
	}

	if report.Title != "Daily Digest" {
		t.Errorf("expected title 'Daily Digest', got %s", report.Title)
	}
	if len(report.TopStories) != 1 {
		t.Errorf("expected 1 top story, got %d", len(report.TopStories))
	}
	if report.Stats.TotalArticles != 10 {
		t.Errorf("expected 10 total articles, got %d", report.Stats.TotalArticles)
	}
}

func TestStats(t *testing.T) {
	stats := Stats{
		TotalArticles: 25,
		BySource: map[string]int{
			"hackernews": 15,
			"rss":        10,
		},
		AvgImportance: 6.8,
	}

	if stats.TotalArticles != 25 {
		t.Errorf("expected 25 total articles, got %d", stats.TotalArticles)
	}
	if stats.BySource["hackernews"] != 15 {
		t.Errorf("expected 15 from hackernews, got %d", stats.BySource["hackernews"])
	}
	if stats.AvgImportance != 6.8 {
		t.Errorf("expected avg importance 6.8, got %f", stats.AvgImportance)
	}
}
