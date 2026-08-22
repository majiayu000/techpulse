package summarizer

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/majiayu000/techpulse/internal/collector"
	"github.com/majiayu000/techpulse/internal/filter"
	"github.com/majiayu000/techpulse/internal/httpclient"
	"github.com/majiayu000/techpulse/internal/logger"
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

	client := httpclient.New()
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

// recordingLogger captures warn-level messages so tests can assert that
// extraction failures are surfaced through the log.
type recordingLogger struct {
	warnMsgs []string
}

func (l *recordingLogger) Debug(msg string, fields ...logger.Field) {}
func (l *recordingLogger) Info(msg string, fields ...logger.Field)  {}
func (l *recordingLogger) Warn(msg string, fields ...logger.Field) {
	l.warnMsgs = append(l.warnMsgs, msg)
}
func (l *recordingLogger) Error(msg string, fields ...logger.Field) {}
func (l *recordingLogger) WithFields(fields ...logger.Field) logger.Logger {
	return l
}

// setRecordingLogger swaps in a recording logger and restores the previous
// default when the test finishes.
func setRecordingLogger(t *testing.T) *recordingLogger {
	t.Helper()
	rec := &recordingLogger{}
	prev := logger.Default()
	logger.SetDefault(rec)
	t.Cleanup(func() { logger.SetDefault(prev) })
	return rec
}

func TestSummarizingEnricher_ExtractFailureCountedAndLogged(t *testing.T) {
	rec := setRecordingLogger(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	client := httpclient.New()
	enricher := NewSummarizingEnricher(client, DefaultSummaryConfig())

	articles := []filter.FilteredArticle{
		// Has content: summary comes from existing content without any fetch,
		// so extraction succeeds.
		{Article: collector.Article{
			ID:      "with-content",
			Title:   "With Content",
			URL:     "https://example.com/a",
			Content: "<p>Existing body text.</p>",
			Score:   100,
		}},
		// No content: extraction fetches the URL, gets a 404, yields "".
		{Article: makeArticle("dead link", server.URL, 100)},
	}

	result, err := enricher.Enrich(context.Background(), articles)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 articles, got %d", len(result))
	}

	byID := map[string]EnrichedArticle{}
	for _, a := range result {
		byID[a.ID] = a
	}
	if byID["with-content"].Summary == "" {
		t.Error("expected non-empty summary for article with existing content")
	}
	if byID["test-id"].Summary != "" {
		t.Error("expected empty summary for failed extraction")
	}

	report, err := enricher.GenerateReport(context.Background(), result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Stats.SummaryFailures != 1 {
		t.Errorf("Stats.SummaryFailures = %d, want 1", report.Stats.SummaryFailures)
	}
	if len(rec.warnMsgs) == 0 {
		t.Error("expected a warning log line for failed extractions")
	}
}

func TestSummarizingEnricher_NoFailuresWhenDisabled(t *testing.T) {
	setRecordingLogger(t)

	enricher := NewSummarizingEnricher(httpclient.New(), SummaryConfig{Enabled: false})

	articles := []filter.FilteredArticle{
		{Article: makeArticle("test", "http://127.0.0.1:1/x", 100)}, // never fetched
	}

	result, err := enricher.Enrich(context.Background(), articles)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	report, err := enricher.GenerateReport(context.Background(), result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Stats.SummaryFailures != 0 {
		t.Errorf("Stats.SummaryFailures = %d, want 0 when summaries are disabled", report.Stats.SummaryFailures)
	}
}

func TestSummarizingEnricher_FailureCountResetsBetweenRuns(t *testing.T) {
	setRecordingLogger(t)

	failServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer failServer.Close()

	okServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("<html><body><p>Fresh content for run two.</p></body></html>"))
	}))
	defer okServer.Close()

	enricher := NewSummarizingEnricher(httpclient.New(), DefaultSummaryConfig())

	run1, err := enricher.Enrich(context.Background(), []filter.FilteredArticle{
		{Article: makeArticle("dead link", failServer.URL, 100)},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	report1, err := enricher.GenerateReport(context.Background(), run1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report1.Stats.SummaryFailures != 1 {
		t.Errorf("run 1: Stats.SummaryFailures = %d, want 1", report1.Stats.SummaryFailures)
	}

	run2, err := enricher.Enrich(context.Background(), []filter.FilteredArticle{
		{Article: makeArticle("ok link", okServer.URL, 100)},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	report2, err := enricher.GenerateReport(context.Background(), run2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report2.Stats.SummaryFailures != 0 {
		t.Errorf("run 2: Stats.SummaryFailures = %d, want 0 after successful rerun", report2.Stats.SummaryFailures)
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
