package filter

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"

	"github.com/anthropic/autonomous-runner/internal/collector"
)

// DedupFilter removes duplicate articles based on title similarity.
type DedupFilter struct {
	seen       map[string]string // hash -> article ID
	similarity float64           // similarity threshold (0-1)
}

// NewDedupFilter creates a new deduplication filter.
func NewDedupFilter(similarity float64) *DedupFilter {
	if similarity <= 0 || similarity > 1 {
		similarity = 0.8
	}
	return &DedupFilter{
		seen:       make(map[string]string),
		similarity: similarity,
	}
}

// Name returns the filter's name.
func (f *DedupFilter) Name() string {
	return "dedup"
}

// Apply removes duplicate articles.
func (f *DedupFilter) Apply(articles []collector.Article) []FilteredArticle {
	result := make([]FilteredArticle, 0, len(articles))

	for _, a := range articles {
		hash := f.computeHash(a)

		filtered := FilteredArticle{Article: a}

		if existingID, exists := f.seen[hash]; exists {
			filtered.IsDuplicate = true
			filtered.DuplicateOf = existingID
		} else {
			f.seen[hash] = a.ID
		}

		// Only include non-duplicates
		if !filtered.IsDuplicate {
			result = append(result, filtered)
		}
	}

	return result
}

// ApplyToFiltered applies deduplication to already filtered articles.
func (f *DedupFilter) ApplyToFiltered(articles []FilteredArticle) []FilteredArticle {
	result := make([]FilteredArticle, 0, len(articles))

	for _, a := range articles {
		hash := f.computeHash(a.Article)

		if existingID, exists := f.seen[hash]; exists {
			a.IsDuplicate = true
			a.DuplicateOf = existingID
		} else {
			f.seen[hash] = a.ID
		}

		if !a.IsDuplicate {
			result = append(result, a)
		}
	}

	return result
}

// Reset clears the seen cache.
func (f *DedupFilter) Reset() {
	f.seen = make(map[string]string)
}

func (f *DedupFilter) computeHash(a collector.Article) string {
	// Normalize the title for comparison
	normalized := strings.ToLower(strings.TrimSpace(a.Title))
	normalized = regexp.MustCompile(`\s+`).ReplaceAllString(normalized, " ")
	normalized = regexp.MustCompile(`[^\w\s]`).ReplaceAllString(normalized, "")

	hash := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(hash[:8]) // Use first 8 bytes
}
