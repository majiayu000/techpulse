package summarizer

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/majiayu000/techpulse/internal/collector"
	"github.com/majiayu000/techpulse/internal/filter"
)

func TestComputeStats(t *testing.T) {
	s := NewBasicSummarizer()
	articles := []EnrichedArticle{
		{FilteredArticle: filter.FilteredArticle{Article: collector.Article{Source: "hn"}}, Importance: 8.0},
		{FilteredArticle: filter.FilteredArticle{Article: collector.Article{Source: "hn"}}, Importance: 6.0},
		{FilteredArticle: filter.FilteredArticle{Article: collector.Article{Source: "rss"}}, Importance: 7.0},
	}

	stats := s.computeStats(articles)

	if stats.TotalArticles != 3 {
		t.Errorf("expected 3 total articles, got %d", stats.TotalArticles)
	}
	if stats.BySource["hn"] != 2 {
		t.Errorf("expected 2 from hn, got %d", stats.BySource["hn"])
	}
	if stats.BySource["rss"] != 1 {
		t.Errorf("expected 1 from rss, got %d", stats.BySource["rss"])
	}
	if stats.AvgImportance != 7.0 {
		t.Errorf("expected avg importance 7.0, got %f", stats.AvgImportance)
	}
}

func TestComputeStats_Empty(t *testing.T) {
	s := NewBasicSummarizer()
	stats := s.computeStats(nil)

	if stats.TotalArticles != 0 {
		t.Errorf("expected 0 total articles, got %d", stats.TotalArticles)
	}
	if stats.AvgImportance != 0.0 {
		t.Errorf("expected avg importance 0.0, got %f", stats.AvgImportance)
	}
}

func TestTopN(t *testing.T) {
	s := NewBasicSummarizer()

	articles := make([]EnrichedArticle, 15)
	for i := range articles {
		articles[i] = EnrichedArticle{
			FilteredArticle: filter.FilteredArticle{
				Article: collector.Article{ID: string(rune('a' + i))},
			},
		}
	}

	top := s.topN(articles, 10)
	if len(top) != 10 {
		t.Errorf("expected 10 articles, got %d", len(top))
	}
}

func TestTopN_LessThanN(t *testing.T) {
	s := NewBasicSummarizer()

	articles := make([]EnrichedArticle, 5)
	top := s.topN(articles, 10)
	if len(top) != 5 {
		t.Errorf("expected 5 articles, got %d", len(top))
	}
}

func TestGenerateReport(t *testing.T) {
	s := NewBasicSummarizer()
	articles := []EnrichedArticle{
		{
			FilteredArticle: filter.FilteredArticle{
				Article: collector.Article{
					ID:     "1",
					Title:  "AI Revolution",
					URL:    "https://example.com/ai",
					Source: "hackernews",
					Score:  500,
				},
				MatchedKeywords: []string{"AI"},
			},
			Importance: 9.0,
		},
	}

	report, err := s.GenerateReport(context.Background(), articles)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if report.Title != "Daily Tech Digest" {
		t.Errorf("expected title 'Daily Tech Digest', got %s", report.Title)
	}
	if len(report.TopStories) != 1 {
		t.Errorf("expected 1 top story, got %d", len(report.TopStories))
	}
	if report.Stats.TotalArticles != 1 {
		t.Errorf("expected 1 total article, got %d", report.Stats.TotalArticles)
	}
	if !strings.Contains(report.Content, "AI Revolution") {
		t.Error("expected content to contain 'AI Revolution'")
	}
	if !strings.Contains(report.Content, "9.0/10") {
		t.Error("expected content to contain importance score")
	}
}

func TestGenerateMarkdown(t *testing.T) {
	s := NewBasicSummarizer()
	articles := []EnrichedArticle{
		{
			FilteredArticle: filter.FilteredArticle{
				Article: collector.Article{
					Title:  "Test Article",
					URL:    "https://example.com",
					Source: "test",
					Score:  100,
				},
				MatchedKeywords: []string{"Go", "Rust"},
			},
			Importance: 7.5,
		},
	}
	stats := Stats{TotalArticles: 1, AvgImportance: 7.5}

	md := s.generateMarkdown("2026-01-01", articles, stats)

	if !strings.Contains(md, "# Daily Tech Digest - 2026-01-01") {
		t.Error("expected header with date")
	}
	if !strings.Contains(md, "**1** articles") {
		t.Error("expected article count")
	}
	if !strings.Contains(md, "[Test Article](https://example.com)") {
		t.Error("expected article link")
	}
	if !strings.Contains(md, "Keywords: [Go Rust]") {
		t.Error("expected keywords")
	}
}

func TestGenerateMarkdown_WithPublishedDate(t *testing.T) {
	s := NewBasicSummarizer()
	pubTime := time.Date(2026, 1, 1, 10, 30, 0, 0, time.UTC)
	articles := []EnrichedArticle{
		{
			FilteredArticle: filter.FilteredArticle{
				Article: collector.Article{
					Title:       "Article With Date",
					URL:         "https://example.com/dated",
					Source:      "rss",
					Score:       50,
					PublishedAt: pubTime,
				},
			},
			Importance: 6.0,
		},
	}
	stats := Stats{TotalArticles: 1, AvgImportance: 6.0}

	md := s.generateMarkdown("2026-01-01", articles, stats)

	if !strings.Contains(md, "Published: 2026-01-01 10:30") {
		t.Error("expected published date in markdown")
	}
}

func TestGenerateMarkdown_WithoutPublishedDate(t *testing.T) {
	s := NewBasicSummarizer()
	articles := []EnrichedArticle{
		{
			FilteredArticle: filter.FilteredArticle{
				Article: collector.Article{
					Title:  "Article Without Date",
					URL:    "https://example.com/nodated",
					Source: "test",
					Score:  50,
				},
			},
			Importance: 6.0,
		},
	}
	stats := Stats{TotalArticles: 1, AvgImportance: 6.0}

	md := s.generateMarkdown("2026-01-01", articles, stats)

	if strings.Contains(md, "Published:") {
		t.Error("expected no published date when time is zero")
	}
}
