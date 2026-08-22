package filter

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/majiayu000/techpulse/internal/collector"
)

// KeywordFilter filters articles based on keyword matching.
type KeywordFilter struct {
	include  []string
	exclude  []string
	patterns []keywordPattern
}

// keywordPattern pairs a compiled include pattern with the keyword that
// produced it, keeping MatchedKeywords aligned even if some patterns are
// skipped at compile time.
type keywordPattern struct {
	keyword string
	re      *regexp.Regexp
}

// NewKeywordFilter creates a new keyword filter.
//
// Single-word keywords are compiled with word boundaries ((?i)\bkw\b) so
// they only match standalone tokens: without boundaries "Go" matched
// "Google", "AI" matched the "ai" inside "email"/"said", letting nearly
// every English article through. The deliberate trade-off is that compound
// tech words do NOT trigger their parts — "Golang" does not match "Go",
// "OpenAI" does not match "AI" — because bare-substring matching flooded
// results with false positives (each one inflating importance); such
// products should get their own keyword instead. Multi-word phrase keywords
// stay unanchored substrings ("large language model" still matches
// "Large Language Models") since phrases are already specific enough.
func NewKeywordFilter(include, exclude []string) *KeywordFilter {
	f := &KeywordFilter{
		include: include,
		exclude: exclude,
	}

	for _, kw := range include {
		if pattern := compileIncludePattern(kw); pattern != nil {
			f.patterns = append(f.patterns, keywordPattern{keyword: kw, re: pattern})
		}
	}

	return f
}

// compileIncludePattern compiles an include keyword to a case-insensitive
// regexp, anchored with \b on the sides that start/end with a word rune.
// It returns nil if the keyword cannot be compiled.
func compileIncludePattern(kw string) *regexp.Regexp {
	isWordRune := func(r rune) bool { return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) }

	first, _ := utf8.DecodeRuneInString(kw)
	last, _ := utf8.DecodeLastRuneInString(kw)
	phrase := strings.IndexFunc(kw, unicode.IsSpace) >= 0

	var b strings.Builder
	b.WriteString("(?i)")
	if !phrase && first != utf8.RuneError && isWordRune(first) {
		b.WriteString(`\b`)
	}
	b.WriteString(regexp.QuoteMeta(kw))
	if !phrase && last != utf8.RuneError && isWordRune(last) {
		b.WriteString(`\b`)
	}

	pattern, err := regexp.Compile(b.String())
	if err != nil {
		return nil
	}
	return pattern
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

	for _, p := range f.patterns {
		if p.re.MatchString(text) {
			matched = append(matched, p.keyword)
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
