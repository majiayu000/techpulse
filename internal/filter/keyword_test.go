package filter

import (
	"testing"

	"github.com/anthropic/autonomous-runner/internal/collector"
)

func TestKeywordFilter_Name(t *testing.T) {
	f := NewKeywordFilter(nil, nil)
	if f.Name() != "keyword" {
		t.Errorf("expected name 'keyword', got '%s'", f.Name())
	}
}

func TestKeywordFilter_MatchInclude(t *testing.T) {
	include := []string{"AI", "LLM", "GPT"}
	f := NewKeywordFilter(include, nil)

	articles := []collector.Article{
		createTestArticle("1", "Introduction to AI", ""),
		createTestArticle("2", "How to cook pasta", ""),
		createTestArticle("3", "GPT models explained", ""),
	}

	result := f.Apply(articles)

	if len(result) != 2 {
		t.Errorf("expected 2 matched articles, got %d", len(result))
	}

	if result[0].ID != "1" || result[1].ID != "3" {
		t.Errorf("unexpected article IDs: %s, %s", result[0].ID, result[1].ID)
	}
}

func TestKeywordFilter_MatchExclude(t *testing.T) {
	include := []string{"AI"}
	exclude := []string{"crypto", "bitcoin"}
	f := NewKeywordFilter(include, exclude)

	articles := []collector.Article{
		createTestArticle("1", "AI in healthcare", ""),
		createTestArticle("2", "AI and crypto trading", ""),
		createTestArticle("3", "AI bitcoin analysis", ""),
	}

	result := f.Apply(articles)

	if len(result) != 1 {
		t.Errorf("expected 1 matched article, got %d", len(result))
	}

	if result[0].ID != "1" {
		t.Errorf("expected ID '1', got '%s'", result[0].ID)
	}
}

func TestKeywordFilter_CaseInsensitive(t *testing.T) {
	include := []string{"LLM"}
	f := NewKeywordFilter(include, nil)

	articles := []collector.Article{
		createTestArticle("1", "llm training guide", ""),
		createTestArticle("2", "LLM architecture", ""),
		createTestArticle("3", "Llm comparison", ""),
	}

	result := f.Apply(articles)

	if len(result) != 3 {
		t.Errorf("expected 3 matched articles (case insensitive), got %d", len(result))
	}
}

func TestKeywordFilter_MatchInContent(t *testing.T) {
	include := []string{"Claude"}
	f := NewKeywordFilter(include, nil)

	articles := []collector.Article{
		createTestArticle("1", "Regular title", "This article mentions Claude AI"),
		createTestArticle("2", "Another title", "No relevant content"),
	}

	result := f.Apply(articles)

	if len(result) != 1 {
		t.Errorf("expected 1 matched article, got %d", len(result))
	}

	if result[0].ID != "1" {
		t.Errorf("expected ID '1', got '%s'", result[0].ID)
	}
}

func TestKeywordFilter_MatchedKeywords(t *testing.T) {
	include := []string{"AI", "LLM", "GPT"}
	f := NewKeywordFilter(include, nil)

	articles := []collector.Article{
		createTestArticle("1", "AI and LLM comparison", "Using GPT"),
	}

	result := f.Apply(articles)

	if len(result) != 1 {
		t.Fatalf("expected 1 matched article, got %d", len(result))
	}

	if len(result[0].MatchedKeywords) != 3 {
		t.Errorf("expected 3 matched keywords, got %d", len(result[0].MatchedKeywords))
	}
}

func TestKeywordFilter_EmptyInclude(t *testing.T) {
	f := NewKeywordFilter(nil, nil)

	articles := []collector.Article{
		createTestArticle("1", "Any article", ""),
		createTestArticle("2", "Another article", ""),
	}

	result := f.Apply(articles)

	// With no include keywords, all articles pass
	if len(result) != 2 {
		t.Errorf("expected 2 articles (no filter), got %d", len(result))
	}
}

func TestDefaultKeywords(t *testing.T) {
	include, exclude := DefaultKeywords()

	if len(include) == 0 {
		t.Error("expected non-empty include keywords")
	}

	if len(exclude) == 0 {
		t.Error("expected non-empty exclude keywords")
	}

	// Check some expected keywords
	found := false
	for _, kw := range include {
		if kw == "AI" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'AI' in include keywords")
	}
}

func TestKeywordFilter_PhraseMatching(t *testing.T) {
	include := []string{"large language model", "machine learning", "neural network"}
	f := NewKeywordFilter(include, nil)

	articles := []collector.Article{
		createTestArticle("1", "Intro to Large Language Models", ""),
		createTestArticle("2", "Machine Learning basics", ""),
		createTestArticle("3", "LLM explained", ""), // Should NOT match
		createTestArticle("4", "Neural Network architecture", ""),
	}

	result := f.Apply(articles)

	if len(result) != 3 {
		t.Errorf("expected 3 matched articles for phrases, got %d", len(result))
	}

	// Verify specific matches
	ids := make(map[string]bool)
	for _, r := range result {
		ids[r.ID] = true
	}
	if ids["3"] {
		t.Error("article 3 should not match (LLM != large language model)")
	}
}

func TestKeywordFilter_ExcludeJobPostings(t *testing.T) {
	include := []string{"AI"}
	exclude := []string{"hiring", "job opening", "we're hiring"}
	f := NewKeywordFilter(include, exclude)

	articles := []collector.Article{
		createTestArticle("1", "AI Research Breakthrough", ""),
		createTestArticle("2", "AI Company is Hiring Engineers", ""),
		createTestArticle("3", "New AI Job Opening at Google", ""),
		createTestArticle("4", "We're Hiring AI Researchers", ""),
	}

	result := f.Apply(articles)

	if len(result) != 1 {
		t.Errorf("expected 1 article (job posts excluded), got %d", len(result))
	}

	if result[0].ID != "1" {
		t.Errorf("expected only article 1 to pass, got %s", result[0].ID)
	}
}

func TestKeywordFilter_ExcludeAdsSpam(t *testing.T) {
	include := []string{"AI"}
	exclude := []string{"sponsored", "advertisement", "promoted"}
	f := NewKeywordFilter(include, exclude)

	articles := []collector.Article{
		createTestArticle("1", "AI in Healthcare", ""),
		createTestArticle("2", "[Sponsored] AI Product Review", ""),
		createTestArticle("3", "AI Advertisement Platform", ""),
	}

	result := f.Apply(articles)

	if len(result) != 1 {
		t.Errorf("expected 1 article (ads excluded), got %d", len(result))
	}
}

func TestDefaultKeywords_ContainsPhrases(t *testing.T) {
	include, _ := DefaultKeywords()

	phrases := []string{"large language model", "machine learning", "neural network"}
	for _, phrase := range phrases {
		found := false
		for _, kw := range include {
			if kw == phrase {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected phrase '%s' in default include keywords", phrase)
		}
	}
}

func TestDefaultKeywords_ContainsNegativeFilters(t *testing.T) {
	_, exclude := DefaultKeywords()

	negative := []string{"hiring", "sponsored", "crypto"}
	for _, neg := range negative {
		found := false
		for _, kw := range exclude {
			if kw == neg {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected '%s' in default exclude keywords", neg)
		}
	}
}
