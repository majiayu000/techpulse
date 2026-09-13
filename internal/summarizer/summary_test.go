package summarizer

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/anthropic/autonomous-runner/internal/collector"
	"github.com/anthropic/autonomous-runner/internal/filter"
	"github.com/anthropic/autonomous-runner/internal/httpclient"
)

func makeArticle(title, url string, score int) collector.Article {
	return collector.Article{
		ID:    "test-id",
		Title: title,
		URL:   url,
		Score: score,
	}
}

func TestDefaultSummaryConfig(t *testing.T) {
	cfg := DefaultSummaryConfig()
	if !cfg.Enabled {
		t.Error("Enabled should be true by default")
	}
	if cfg.MaxLength != 200 {
		t.Errorf("MaxLength = %d, want 200", cfg.MaxLength)
	}
	if cfg.Concurrency != 5 {
		t.Errorf("Concurrency = %d, want 5", cfg.Concurrency)
	}
}

func TestSummarizingEnricher_EnrichDisabled(t *testing.T) {
	client := httpclient.New()
	cfg := SummaryConfig{Enabled: false}
	enricher := NewSummarizingEnricher(client, cfg)

	articles := []filter.FilteredArticle{
		{Article: makeArticle("test", "http://example.com", 100)},
	}

	result, err := enricher.Enrich(context.Background(), articles)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 article, got %d", len(result))
	}

	// Summary should be empty when disabled
	if result[0].Summary != "" {
		t.Errorf("expected empty summary, got %q", result[0].Summary)
	}
}

func TestSummarizingEnricher_EnrichWithSummaries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("<html><body><p>This is the article content.</p></body></html>"))
	}))
	defer server.Close()

	client := httpclient.New(httpclient.WithAllowPrivateHosts(true))
	cfg := DefaultSummaryConfig()
	enricher := NewSummarizingEnricher(client, cfg)

	articles := []filter.FilteredArticle{
		{Article: makeArticle("test", server.URL, 100)},
	}

	result, err := enricher.Enrich(context.Background(), articles)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 article, got %d", len(result))
	}

	// Summary should contain extracted content
	if result[0].Summary == "" {
		t.Error("expected non-empty summary")
	}
}

func TestSummarizingEnricher_GenerateReport(t *testing.T) {
	client := httpclient.New()
	cfg := SummaryConfig{Enabled: false}
	enricher := NewSummarizingEnricher(client, cfg)

	articles := []EnrichedArticle{
		{
			FilteredArticle: filter.FilteredArticle{
				Article: makeArticle("Test Article", "http://example.com", 100),
			},
			Importance: 7.5,
			Summary:    "This is a test summary",
		},
	}

	report, err := enricher.GenerateReport(context.Background(), articles)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.Title != "Daily Tech Digest" {
		t.Errorf("expected 'Daily Tech Digest', got %q", report.Title)
	}

	// Report content should include summary
	if report.Content == "" {
		t.Error("expected non-empty report content")
	}
}

func TestMin(t *testing.T) {
	if min(5, 10) != 5 {
		t.Error("min(5, 10) should be 5")
	}
	if min(10, 5) != 5 {
		t.Error("min(10, 5) should be 5")
	}
}
