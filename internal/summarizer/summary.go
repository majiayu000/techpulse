package summarizer

import (
	"context"
	"sync"

	"github.com/anthropic/autonomous-runner/internal/extractor"
	"github.com/anthropic/autonomous-runner/internal/filter"
	"github.com/anthropic/autonomous-runner/internal/httpclient"
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

			summary, _ := s.extractor.Extract(ctx, articles[idx].URL, articles[idx].Content)
			articles[idx].Summary = summary
		}(i)
	}

	wg.Wait()
}

// GenerateReport creates a report with summaries.
func (s *SummarizingEnricher) GenerateReport(ctx context.Context, articles []EnrichedArticle) (Report, error) {
	return s.base.GenerateReport(ctx, articles)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
