package summarizer

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/anthropic/autonomous-runner/internal/filter"
)

// BasicSummarizer provides a simple scoring implementation without AI.
type BasicSummarizer struct {
	sourceWeights *SourceWeights
}

// NewBasicSummarizer creates a new basic summarizer.
func NewBasicSummarizer() *BasicSummarizer {
	return &BasicSummarizer{
		sourceWeights: NewSourceWeights(),
	}
}

// Enrich adds importance scores to articles based on metadata.
func (s *BasicSummarizer) Enrich(ctx context.Context, articles []filter.FilteredArticle) ([]EnrichedArticle, error) {
	result := make([]EnrichedArticle, 0, len(articles))

	for _, a := range articles {
		enriched := EnrichedArticle{
			FilteredArticle: a,
			Importance:      s.calculateImportance(a),
		}
		result = append(result, enriched)
	}

	// Sort by importance descending
	sort.Slice(result, func(i, j int) bool {
		return result[i].Importance > result[j].Importance
	})

	return result, nil
}

func (s *BasicSummarizer) calculateImportance(a filter.FilteredArticle) float64 {
	score := 5.0 // Base score

	// Adjust based on article score
	if a.Score > 500 {
		score += 2.0
	} else if a.Score > 100 {
		score += 1.0
	} else if a.Score > 50 {
		score += 0.5
	}

	// Adjust based on comments
	if a.Comments > 200 {
		score += 1.5
	} else if a.Comments > 50 {
		score += 0.5
	}

	// Bonus for matched keywords
	score += float64(len(a.MatchedKeywords)) * 0.5

	// Bonus for source weight (trusted sources get higher scores)
	score += s.sourceWeights.CalculateBonus(a.Source)

	// Cap at 10, floor at 1
	if score > 10 {
		score = 10
	}
	if score < 1 {
		score = 1
	}

	return score
}

// GenerateReport creates a Markdown report from enriched articles.
func (s *BasicSummarizer) GenerateReport(ctx context.Context, articles []EnrichedArticle) (Report, error) {
	date := time.Now().Format("2006-01-02")
	stats := s.computeStats(articles)

	content := s.generateMarkdown(date, articles, stats)

	return Report{
		Title:      "Daily Tech Digest",
		Date:       date,
		TopStories: s.topN(articles, 10),
		Stats:      stats,
		Content:    content,
	}, nil
}

func (s *BasicSummarizer) computeStats(articles []EnrichedArticle) Stats {
	bySource := make(map[string]int)
	var totalImportance float64

	for _, a := range articles {
		bySource[a.Source]++
		totalImportance += a.Importance
	}

	avgImportance := 0.0
	if len(articles) > 0 {
		avgImportance = totalImportance / float64(len(articles))
	}

	return Stats{
		TotalArticles: len(articles),
		BySource:      bySource,
		AvgImportance: avgImportance,
	}
}

func (s *BasicSummarizer) topN(articles []EnrichedArticle, n int) []EnrichedArticle {
	if len(articles) <= n {
		return articles
	}
	return articles[:n]
}

func (s *BasicSummarizer) generateMarkdown(date string, articles []EnrichedArticle, stats Stats) string {
	md := fmt.Sprintf("# Daily Tech Digest - %s\n\n", date)
	md += fmt.Sprintf("Collected **%d** articles | Avg Importance: **%.1f**/10\n\n",
		stats.TotalArticles, stats.AvgImportance)

	md += "## Top Stories\n\n"
	for i, a := range articles {
		if i >= 10 {
			break
		}
		md += fmt.Sprintf("### %d. [%s](%s)\n", i+1, a.Title, a.URL)
		md += s.formatArticleMeta(a)
		if len(a.MatchedKeywords) > 0 {
			md += fmt.Sprintf("- Keywords: %v\n", a.MatchedKeywords)
		}
		if a.Summary != "" {
			md += fmt.Sprintf("\n> %s\n", a.Summary)
		}
		md += "\n"
	}

	return md
}

func (s *BasicSummarizer) formatArticleMeta(a EnrichedArticle) string {
	meta := fmt.Sprintf("- Source: %s | Score: %d | Importance: %.1f/10",
		a.Source, a.Score, a.Importance)
	if !a.PublishedAt.IsZero() {
		meta += fmt.Sprintf(" | Published: %s", a.PublishedAt.Format("2006-01-02 15:04"))
	}
	return meta + "\n"
}
