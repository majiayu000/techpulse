// Package filter provides filtering capabilities for collected articles.
package filter

import (
	"github.com/majiayu000/techpulse/internal/collector"
)

// FilteredArticle represents an article with filtering metadata.
type FilteredArticle struct {
	collector.Article
	MatchedKeywords []string `json:"matched_keywords,omitempty"`
	IsDuplicate     bool     `json:"is_duplicate"`
	DuplicateOf     string   `json:"duplicate_of,omitempty"`
	BaseScore       float64  `json:"base_score"`
}

// Filter defines the interface for article filters.
type Filter interface {
	// Name returns the filter's name.
	Name() string

	// Apply filters the articles and returns filtered results.
	Apply(articles []collector.Article) []FilteredArticle
}

// ToFiltered converts raw articles to filtered articles.
func ToFiltered(articles []collector.Article) []FilteredArticle {
	result := make([]FilteredArticle, len(articles))
	for i, a := range articles {
		result[i] = FilteredArticle{Article: a}
	}
	return result
}

// ToArticles extracts raw articles from filtered articles.
func ToArticles(filtered []FilteredArticle) []collector.Article {
	result := make([]collector.Article, len(filtered))
	for i, f := range filtered {
		result[i] = f.Article
	}
	return result
}
