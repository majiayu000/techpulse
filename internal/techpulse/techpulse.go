// Package techpulse provides the main orchestrator for tech news collection.
package techpulse

import (
	"context"
	"fmt"
	"time"

	"github.com/anthropic/autonomous-runner/internal/collector"
	"github.com/anthropic/autonomous-runner/internal/filter"
	"github.com/anthropic/autonomous-runner/internal/httpclient"
	"github.com/anthropic/autonomous-runner/internal/logger"
	"github.com/anthropic/autonomous-runner/internal/storage"
	"github.com/anthropic/autonomous-runner/internal/summarizer"
)

// Config contains TechPulse configuration options.
type Config struct {
	Limit         int              // Maximum articles per source
	Sources       []string         // Specific sources to use (empty = all)
	Output        string           // Custom output directory
	Timeout       int              // Request timeout in seconds
	Keywords      *KeywordsConfig  // Custom keyword filters
	RSSFeeds      []RSSFeedConfig  // Custom RSS feeds
	EnableSummary bool             // Enable content summary extraction
}

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	return Config{
		Limit:   30,
		Sources: nil,
		Output:  ".techpulse",
		Timeout: 60,
	}
}

// TechPulse orchestrates the collection, filtering, and storage of tech news.
type TechPulse struct {
	config      Config
	registry    *collector.Registry
	pipeline    *filter.Pipeline
	summarizer  summarizer.Summarizer
	storage     storage.Storage
	log         logger.Logger
	showProgress bool
}

// New creates a new TechPulse instance with default configuration.
func New() *TechPulse {
	return NewWithOptions(DefaultConfig())
}

// NewWithOptions creates a TechPulse instance with custom options.
func NewWithOptions(cfg Config) *TechPulse {
	storeCfg := storage.DefaultConfig()
	if cfg.Output != "" {
		storeCfg.BaseDir = cfg.Output
	}

	return &TechPulse{
		config:     cfg,
		registry:   buildRegistry(cfg),
		pipeline:   buildFilterPipeline(cfg.Keywords),
		summarizer: buildSummarizer(cfg),
		storage:    storage.NewMarkdownStorage(storeCfg),
		log:        logger.Default(),
	}
}

// buildSummarizer creates the appropriate summarizer based on config.
func buildSummarizer(cfg Config) summarizer.Summarizer {
	if cfg.EnableSummary {
		client := httpclient.New()
		return summarizer.NewSummarizingEnricher(client, summarizer.DefaultSummaryConfig())
	}
	return summarizer.NewBasicSummarizer()
}

// buildFilterPipeline creates filter pipeline with optional custom keywords.
func buildFilterPipeline(kw *KeywordsConfig) *filter.Pipeline {
	if kw == nil || (len(kw.Include) == 0 && len(kw.Exclude) == 0) {
		return filter.DefaultPipeline()
	}

	include := kw.Include
	exclude := kw.Exclude
	if len(include) == 0 {
		include, _ = filter.DefaultKeywords()
	}
	if len(exclude) == 0 {
		_, exclude = filter.DefaultKeywords()
	}

	return filter.NewPipeline(
		filter.NewKeywordFilter(include, exclude),
	).WithDedup(0.8)
}

// AvailableSources returns the names of all available data sources.
func (tp *TechPulse) AvailableSources() []string {
	return tp.registry.Names()
}

// SetLogger sets a custom logger.
func (tp *TechPulse) SetLogger(l logger.Logger) {
	tp.log = l
}

// SetShowProgress enables or disables progress bar display.
func (tp *TechPulse) SetShowProgress(show bool) {
	tp.showProgress = show
}

// Run executes the full collection pipeline.
func (tp *TechPulse) Run(ctx context.Context) error {
	tp.log.Info("TechPulse starting...")
	opts := collector.Options{
		Limit:   tp.config.Limit,
		Timeout: time.Duration(tp.config.Timeout) * time.Second,
	}
	results := tp.collectFromSources(ctx, opts)
	allArticles := tp.combineResults(results)
	if len(allArticles) == 0 {
		return fmt.Errorf("no articles collected")
	}
	tp.log.Info("Total collected", logger.F("count", len(allArticles)))

	tp.log.Info("Filtering...")
	filtered := tp.pipeline.Process(allArticles)
	tp.log.Info("After filtering", logger.F("count", len(filtered)))

	tp.log.Info("Scoring articles...")
	enriched, err := tp.summarizer.Enrich(ctx, filtered)
	if err != nil {
		return fmt.Errorf("enrich articles: %w", err)
	}

	tp.log.Info("Generating report...")
	report, err := tp.summarizer.GenerateReport(ctx, enriched)
	if err != nil {
		return fmt.Errorf("generate report: %w", err)
	}

	tp.log.Info("Saving...")
	if err := tp.storage.Save(enriched); err != nil {
		return fmt.Errorf("save articles: %w", err)
	}
	if err := tp.storage.SaveReport(report); err != nil {
		return fmt.Errorf("save report: %w", err)
	}
	tp.log.Info("Done!", logger.F("articles", len(enriched)))
	return nil
}
