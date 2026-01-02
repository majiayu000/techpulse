package summarizer

import (
	"testing"

	"github.com/anthropic/autonomous-runner/internal/collector"
	"github.com/anthropic/autonomous-runner/internal/filter"
)

func TestCalculateImportance_ScoreThresholds(t *testing.T) {
	s := NewBasicSummarizer()

	tests := []struct {
		name     string
		score    int
		expected float64
	}{
		{"low score", 30, 5.0},
		{"score above 50", 60, 5.5},
		{"score above 100", 150, 6.0},
		{"score above 500", 600, 7.0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := filter.FilteredArticle{
				Article: collector.Article{Score: tc.score},
			}
			importance := s.calculateImportance(a)
			if importance != tc.expected {
				t.Errorf("expected %f, got %f", tc.expected, importance)
			}
		})
	}
}

func TestCalculateImportance_CommentsThresholds(t *testing.T) {
	s := NewBasicSummarizer()

	tests := []struct {
		name     string
		comments int
		expected float64
	}{
		{"few comments", 20, 5.0},
		{"comments above 50", 80, 5.5},
		{"comments above 200", 300, 6.5},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := filter.FilteredArticle{
				Article: collector.Article{Comments: tc.comments},
			}
			importance := s.calculateImportance(a)
			if importance != tc.expected {
				t.Errorf("expected %f, got %f", tc.expected, importance)
			}
		})
	}
}

func TestCalculateImportance_Keywords(t *testing.T) {
	s := NewBasicSummarizer()

	tests := []struct {
		name     string
		keywords []string
		expected float64
	}{
		{"no keywords", nil, 5.0},
		{"one keyword", []string{"AI"}, 5.5},
		{"two keywords", []string{"AI", "GPT"}, 6.0},
		{"three keywords", []string{"AI", "GPT", "LLM"}, 6.5},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := filter.FilteredArticle{
				MatchedKeywords: tc.keywords,
			}
			importance := s.calculateImportance(a)
			if importance != tc.expected {
				t.Errorf("expected %f, got %f", tc.expected, importance)
			}
		})
	}
}

func TestCalculateImportance_Cap(t *testing.T) {
	s := NewBasicSummarizer()
	a := filter.FilteredArticle{
		Article:         collector.Article{Score: 1000, Comments: 500},
		MatchedKeywords: []string{"AI", "GPT", "LLM", "Claude", "Rust", "Go"},
	}

	importance := s.calculateImportance(a)
	if importance != 10.0 {
		t.Errorf("expected capped at 10.0, got %f", importance)
	}
}

func TestCalculateImportance_Combined(t *testing.T) {
	s := NewBasicSummarizer()
	a := filter.FilteredArticle{
		Article:         collector.Article{Score: 200, Comments: 100},
		MatchedKeywords: []string{"AI", "GPT"},
	}
	// Base 5.0 + score>100: 1.0 + comments>50: 0.5 + 2 keywords: 1.0 = 7.5
	importance := s.calculateImportance(a)
	if importance != 7.5 {
		t.Errorf("expected 7.5, got %f", importance)
	}
}

func TestCalculateImportance_SourceWeight(t *testing.T) {
	s := NewBasicSummarizer()

	tests := []struct {
		name     string
		source   string
		expected float64
	}{
		// Base 5.0 + source bonus
		{"hackernews boost", "hackernews_top", 6.0},   // +1.0 bonus
		{"lobsters boost", "lobsters_hottest", 6.0},   // +1.0 bonus
		{"reddit neutral", "reddit", 5.0},             // +0.0 bonus
		{"unknown neutral", "unknown_source", 5.0},    // +0.0 bonus (default)
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := filter.FilteredArticle{
				Article: collector.Article{Source: tc.source},
			}
			importance := s.calculateImportance(a)
			if importance != tc.expected {
				t.Errorf("expected %f, got %f", tc.expected, importance)
			}
		})
	}
}

func TestCalculateImportance_SourceWeightCombined(t *testing.T) {
	s := NewBasicSummarizer()
	// HN article with high score, comments and keywords
	a := filter.FilteredArticle{
		Article: collector.Article{
			Source:   "hackernews_top",
			Score:    600,  // +2.0
			Comments: 300, // +1.5
		},
		MatchedKeywords: []string{"AI", "GPT"}, // +1.0
	}
	// Base 5.0 + score: 2.0 + comments: 1.5 + keywords: 1.0 + source: 1.0 = 10.5 -> capped at 10.0
	importance := s.calculateImportance(a)
	if importance != 10.0 {
		t.Errorf("expected 10.0 (capped), got %f", importance)
	}
}
