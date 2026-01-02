// Package summarizer provides AI-powered article enrichment and summarization.
package summarizer

import (
	"context"

	"github.com/anthropic/autonomous-runner/internal/filter"
)

// EnrichedArticle represents an article with AI-generated enhancements.
type EnrichedArticle struct {
	filter.FilteredArticle
	Importance float64  `json:"importance"` // 0-10 score
	Summary    string   `json:"summary"`
	AITags     []string `json:"ai_tags,omitempty"`
}

// Summarizer defines the interface for article enrichment.
type Summarizer interface {
	// Enrich adds AI-generated scores and summaries to articles.
	Enrich(ctx context.Context, articles []filter.FilteredArticle) ([]EnrichedArticle, error)

	// GenerateReport creates a daily digest report.
	GenerateReport(ctx context.Context, articles []EnrichedArticle) (Report, error)
}

// Report represents a daily digest report.
type Report struct {
	Title      string
	Date       string
	TopStories []EnrichedArticle
	Stats      Stats
	Content    string // Markdown content
}

// Stats contains collection statistics.
type Stats struct {
	TotalArticles int
	BySource      map[string]int
	AvgImportance float64
}
