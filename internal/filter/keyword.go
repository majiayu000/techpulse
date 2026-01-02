package filter

import (
	"regexp"
	"strings"

	"github.com/anthropic/autonomous-runner/internal/collector"
)

// KeywordFilter filters articles based on keyword matching.
type KeywordFilter struct {
	include  []string
	exclude  []string
	patterns []*regexp.Regexp
}

// NewKeywordFilter creates a new keyword filter.
func NewKeywordFilter(include, exclude []string) *KeywordFilter {
	f := &KeywordFilter{
		include: include,
		exclude: exclude,
	}

	// Pre-compile regex patterns for include keywords
	for _, kw := range include {
		pattern, err := regexp.Compile("(?i)" + regexp.QuoteMeta(kw))
		if err == nil {
			f.patterns = append(f.patterns, pattern)
		}
	}

	return f
}

// DefaultKeywords returns common tech/AI keywords including phrases.
func DefaultKeywords() (include, exclude []string) {
	include = []string{
		// Single word keywords
		"AI", "LLM", "GPT", "Claude", "OpenAI", "Anthropic",
		"Rust", "Go", "TypeScript", "Python",
		"Gemini", "Llama", "Mistral", "Copilot",
		// Multi-word phrases (already supported via regex)
		"machine learning", "neural network", "deep learning",
		"large language model", "generative AI", "artificial intelligence",
		"natural language processing", "computer vision",
		"open source", "developer tools",
	}
	exclude = []string{
		// Crypto/Web3
		"crypto", "NFT", "blockchain", "bitcoin", "ethereum", "web3",
		// Ads/Spam
		"sponsored", "advertisement", "promoted",
		// Job postings
		"hiring", "job opening", "we're hiring", "join our team",
		// Unrelated
		"stock price", "market cap", "price prediction",
	}
	return
}

// Name returns the filter's name.
func (f *KeywordFilter) Name() string {
	return "keyword"
}

// Apply filters articles based on keywords.
func (f *KeywordFilter) Apply(articles []collector.Article) []FilteredArticle {
	result := make([]FilteredArticle, 0, len(articles))

	for _, a := range articles {
		matched := f.matchKeywords(a)

		// If include keywords are specified, article must match at least one
		if len(f.include) > 0 && len(matched) == 0 {
			continue
		}

		// Skip if matches any exclude keyword
		if f.matchExclude(a) {
			continue
		}

		result = append(result, FilteredArticle{
			Article:         a,
			MatchedKeywords: matched,
		})
	}

	return result
}

func (f *KeywordFilter) matchKeywords(a collector.Article) []string {
	text := strings.ToLower(a.Title + " " + a.Content)
	var matched []string

	for i, pattern := range f.patterns {
		if pattern.MatchString(text) {
			matched = append(matched, f.include[i])
		}
	}

	return matched
}

func (f *KeywordFilter) matchExclude(a collector.Article) bool {
	text := strings.ToLower(a.Title + " " + a.Content)

	for _, kw := range f.exclude {
		if strings.Contains(text, strings.ToLower(kw)) {
			return true
		}
	}

	return false
}
