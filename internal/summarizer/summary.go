package summarizer

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/majiayu000/techpulse/internal/extractor"
	"github.com/majiayu000/techpulse/internal/filter"
	"github.com/majiayu000/techpulse/internal/httpclient"
	"github.com/majiayu000/techpulse/internal/logger"
)

// SummaryConfig configures summary extraction behavior.
type SummaryConfig struct {
	Enabled     bool // Whether to extract summaries
	MaxLength   int  // Maximum summary length
	Concurrency int  // Number of concurrent extractions
}

// DefaultSummaryConfig returns sensible defaults.
func DefaultSummaryConfig() SummaryConfig {
	return SummaryConfig{
		Enabled:     true,
		MaxLength:   200,
		Concurrency: 5,
	}
}

// SummarizingEnricher wraps a BasicSummarizer and adds summary extraction.
type SummarizingEnricher struct {
	base      *BasicSummarizer
	extractor *extractor.Extractor
	config    SummaryConfig
	// summaryFailures counts articles whose extraction produced no summary
	// during the most recent Enrich run. Extract converts fetch failures to
	// ("", nil), so an empty result is the observable failure signal here.
	summaryFailures atomic.Int64
}

// NewSummarizingEnricher creates an enricher with summary extraction.
func NewSummarizingEnricher(client *httpclient.Client, cfg SummaryConfig) *SummarizingEnricher {
	extCfg := extractor.Config{
		MaxLength:   cfg.MaxLength,
		FetchRemote: cfg.Enabled,
	}
	return &SummarizingEnricher{
		base:      NewBasicSummarizer(),
		extractor: extractor.New(client, extCfg),
		config:    cfg,
	}
}

// Enrich adds importance scores and extracts summaries.
func (s *SummarizingEnricher) Enrich(ctx context.Context, articles []filter.FilteredArticle) ([]EnrichedArticle, error) {
	// First, get basic enrichment
	enriched, err := s.base.Enrich(ctx, articles)
	if err != nil {
		return nil, err
	}

	// Each enrichment run owns its failure window; never leak counts from a
	// previous run into this run's report stats.
	s.summaryFailures.Store(0)

	if !s.config.Enabled {
		return enriched, nil
	}

	// Extract summaries concurrently (only for top N articles)
	topN := min(len(enriched), 20) // Only extract summaries for top 20
	s.extractSummaries(ctx, enriched[:topN])

	return enriched, nil
}

func (s *SummarizingEnricher) extractSummaries(ctx context.Context, articles []EnrichedArticle) {
	concurrency := s.config.Concurrency
	if concurrency <= 0 {
		concurrency = 5
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, concurrency)

	for i := range articles {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			summary, err := s.extractor.Extract(ctx, articles[idx].URL, articles[idx].Content)
			if err != nil || summary == "" {
				// Extract itself converts fetch failures to ("", nil), so an
				// empty result is the observable signal that extraction failed.
				// Keep the non-fatal policy but make it countable and visible.
				s.summaryFailures.Add(1)
			}
			articles[idx].Summary = summary
		}(i)
	}

	wg.Wait()

	if failures := s.summaryFailures.Load(); failures > 0 {
		logger.Warn("summary extraction incomplete",
			logger.F("failed", failures),
			logger.F("attempted", len(articles)))
	}
}

// GenerateReport creates a report with summaries.
func (s *SummarizingEnricher) GenerateReport(ctx context.Context, articles []EnrichedArticle) (Report, error) {
	report, err := s.base.GenerateReport(ctx, articles)
	if err != nil {
		return report, err
	}
	report.Stats.SummaryFailures = int(s.summaryFailures.Load())
	return report, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
