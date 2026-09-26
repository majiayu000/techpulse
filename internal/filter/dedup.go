package filter

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"

	"github.com/majiayu000/techpulse/internal/collector"
)

// Pre-compiled once; compiling per title inside the Apply loop is wasted work.
var (
	titleWhitespaceRe = regexp.MustCompile(`\s+`)
	titleNoiseRe      = regexp.MustCompile(`[^\p{L}\p{N}\s]+`)
)

// DedupFilter removes duplicate articles based on title similarity.
//
// Two passes run for every article:
//  1. exact pass — normalized titles are hashed and compared by hash;
//  2. fuzzy pass — normalized-title token sets are compared with Jaccard
//     similarity against every retained title; a score of at least
//     f.similarity marks the article as a duplicate.
type DedupFilter struct {
	seen       map[string]string // normalized-title hash -> article ID
	titles     []seenTitle       // token sets of retained articles (fuzzy pass)
	similarity float64           // similarity threshold (0-1)
}

// seenTitle holds the normalized tokens of a retained article's title,
// used by the fuzzy similarity pass.
type seenTitle struct {
	id     string
	tokens map[string]struct{}
}

// NewDedupFilter creates a new deduplication filter. Articles whose
// normalized-title token-set Jaccard similarity reaches similarity are
// treated as duplicates of the retained article they match.
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
		filtered := FilteredArticle{Article: a}
		filtered.IsDuplicate, filtered.DuplicateOf = f.admit(a)

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
		a.IsDuplicate, a.DuplicateOf = f.admit(a.Article)

		if !a.IsDuplicate {
			result = append(result, a)
		}
	}

	return result
}

// Reset clears the seen cache.
func (f *DedupFilter) Reset() {
	f.seen = make(map[string]string)
	f.titles = nil
}

// admit decides whether the article duplicates an already-seen one. On a
// duplicate it reports true plus the ID of the retained article; otherwise
// it records the article and reports false.
func (f *DedupFilter) admit(a collector.Article) (duplicate bool, duplicateOf string) {
	normalized := normalizeTitle(a.Title)
	hash := titleHash(normalized)

	// Fast path: exact match after normalization.
	if existingID, exists := f.seen[hash]; exists {
		return true, existingID
	}

	// Fuzzy path: near-duplicate by token-set Jaccard similarity.
	if existingID, similar := f.findSimilar(normalized); similar {
		return true, existingID
	}

	f.seen[hash] = a.ID
	f.titles = append(f.titles, seenTitle{id: a.ID, tokens: titleTokens(normalized)})
	return false, ""
}

// findSimilar scans retained titles linearly (O(n) per incoming article,
// constant-time map lookups per comparison) for one whose token-set Jaccard
// similarity with the normalized title reaches the threshold.
func (f *DedupFilter) findSimilar(normalized string) (string, bool) {
	tokens := titleTokens(normalized)
	if len(tokens) == 0 {
		return "", false
	}
	for _, st := range f.titles {
		if jaccard(tokens, st.tokens) >= f.similarity {
			return st.id, true
		}
	}
	return "", false
}

// normalizeTitle lowercases the title, collapses whitespace, and strips
// everything that is not a Unicode letter, number, or whitespace. The
// Unicode-aware classes matter: an ASCII-only class such as [^\w\s] would
// delete every CJK character, collapsing all Chinese titles onto the same
// empty string and hashing them identically.
func normalizeTitle(title string) string {
	s := strings.ToLower(strings.TrimSpace(title))
	s = titleWhitespaceRe.ReplaceAllString(s, " ")
	return titleNoiseRe.ReplaceAllString(s, "")
}

func titleTokens(normalized string) map[string]struct{} {
	fields := strings.Fields(normalized)
	tokens := make(map[string]struct{}, len(fields))
	for _, tok := range fields {
		tokens[tok] = struct{}{}
	}
	return tokens
}

// jaccard returns the token-set Jaccard similarity |A∩B| / |A∪B|.
// Sets are assumed non-empty (guarded by callers).
func jaccard(a, b map[string]struct{}) float64 {
	small, large := a, b
	if len(small) > len(large) {
		small, large = large, small
	}
	intersection := 0
	for tok := range small {
		if _, ok := large[tok]; ok {
			intersection++
		}
	}
	union := len(a) + len(b) - intersection
	return float64(intersection) / float64(union)
}

func titleHash(normalized string) string {
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:8]) // Use first 8 bytes
}
