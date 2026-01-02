// Package techpulse end-to-end integration tests.
package techpulse

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anthropic/autonomous-runner/internal/collector"
	"github.com/anthropic/autonomous-runner/internal/collector/hackernews"
	"github.com/anthropic/autonomous-runner/internal/collector/rss"
	"github.com/anthropic/autonomous-runner/internal/filter"
	"github.com/anthropic/autonomous-runner/internal/logger"
	"github.com/anthropic/autonomous-runner/internal/storage"
	"github.com/anthropic/autonomous-runner/internal/summarizer"
)

// TestIntegrationFullPipeline tests the complete data flow.
func TestIntegrationFullPipeline(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "techpulse-integration-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	hnServer := createMockHNServer(t)
	defer hnServer.Close()

	rssServer := createMockRSSServer(t)
	defer rssServer.Close()

	reg := collector.NewRegistry()
	hnClient := hackernews.NewClientWithBaseURL(hnServer.URL)
	reg.Register(hackernews.NewWithClient("top", hnClient))
	reg.Register(rss.New([]rss.Source{{Name: "Test", URL: rssServer.URL}}))

	storeCfg := storage.DefaultConfig()
	storeCfg.BaseDir = tmpDir
	tp := &TechPulse{
		config:     Config{Limit: 10, Timeout: 30},
		registry:   reg,
		pipeline:   filter.DefaultPipeline(),
		summarizer: summarizer.NewBasicSummarizer(),
		storage:    storage.NewMarkdownStorage(storeCfg),
		log:        logger.NewNopLogger(),
	}

	ctx := context.Background()
	if err = tp.Run(ctx); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Verify output files
	digestPath := filepath.Join(tmpDir, "DIGEST.md")
	if _, err := os.Stat(digestPath); os.IsNotExist(err) {
		t.Error("DIGEST.md was not created")
	}

	archiveDir := filepath.Join(tmpDir, "archive")
	entries, err := os.ReadDir(archiveDir)
	if err != nil || len(entries) == 0 {
		t.Error("archive files not created")
	}
}

// TestIntegrationFilterPipeline tests that filters are applied correctly.
func TestIntegrationFilterPipeline(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "techpulse-filter-*")
	defer os.RemoveAll(tmpDir)

	hnServer := createMockHNServer(t)
	defer hnServer.Close()

	reg := collector.NewRegistry()
	hnClient := hackernews.NewClientWithBaseURL(hnServer.URL)
	reg.Register(hackernews.NewWithClient("top", hnClient))

	storeCfg := storage.DefaultConfig()
	storeCfg.BaseDir = tmpDir
	tp := &TechPulse{
		config:     Config{Limit: 10, Timeout: 30},
		registry:   reg,
		pipeline:   filter.DefaultPipeline(),
		summarizer: summarizer.NewBasicSummarizer(),
		storage:    storage.NewMarkdownStorage(storeCfg),
		log:        logger.NewNopLogger(),
	}

	ctx := context.Background()
	if err := tp.Run(ctx); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	digestPath := filepath.Join(tmpDir, "DIGEST.md")
	content, err := os.ReadFile(digestPath)
	if err != nil {
		t.Fatalf("failed to read DIGEST.md: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "Go 2.0") && !strings.Contains(contentStr, "Rust") {
		t.Error("filtered articles not found in DIGEST.md")
	}
}

// TestIntegrationSpecificSources tests collecting from specific sources only.
func TestIntegrationSpecificSources(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "techpulse-sources-*")
	defer os.RemoveAll(tmpDir)

	hnServer := createMockHNServer(t)
	defer hnServer.Close()

	rssServer := createMockRSSServer(t)
	defer rssServer.Close()

	reg := collector.NewRegistry()
	hnClient := hackernews.NewClientWithBaseURL(hnServer.URL)
	reg.Register(hackernews.NewWithClient("top", hnClient))
	reg.Register(rss.New([]rss.Source{{Name: "Test", URL: rssServer.URL}}))

	storeCfg := storage.DefaultConfig()
	storeCfg.BaseDir = tmpDir
	tp := &TechPulse{
		config:     Config{Limit: 10, Timeout: 30, Sources: []string{"rss"}},
		registry:   reg,
		pipeline:   filter.DefaultPipeline(),
		summarizer: summarizer.NewBasicSummarizer(),
		storage:    storage.NewMarkdownStorage(storeCfg),
		log:        logger.NewNopLogger(),
	}

	ctx := context.Background()
	if err := tp.Run(ctx); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	digestPath := filepath.Join(tmpDir, "DIGEST.md")
	content, _ := os.ReadFile(digestPath)
	if strings.Contains(string(content), "Go 2.0") {
		t.Error("should not contain HN articles when only RSS source selected")
	}
}

// TestIntegrationNoArticlesError tests error when no articles collected.
func TestIntegrationNoArticlesError(t *testing.T) {
	emptyServer := createEmptyHNServer(t)
	defer emptyServer.Close()

	reg := collector.NewRegistry()
	hnClient := hackernews.NewClientWithBaseURL(emptyServer.URL)
	reg.Register(hackernews.NewWithClient("top", hnClient))

	tmpDir, _ := os.MkdirTemp("", "techpulse-empty-*")
	defer os.RemoveAll(tmpDir)

	storeCfg := storage.DefaultConfig()
	storeCfg.BaseDir = tmpDir
	tp := &TechPulse{
		config:     Config{Limit: 10, Timeout: 30},
		registry:   reg,
		pipeline:   filter.DefaultPipeline(),
		summarizer: summarizer.NewBasicSummarizer(),
		storage:    storage.NewMarkdownStorage(storeCfg),
		log:        logger.NewNopLogger(),
	}

	ctx := context.Background()
	err := tp.Run(ctx)

	if err == nil || !strings.Contains(err.Error(), "no articles") {
		t.Errorf("expected 'no articles' error, got: %v", err)
	}
}

// TestIntegrationUnknownSource tests handling of unknown source names.
func TestIntegrationUnknownSource(t *testing.T) {
	hnServer := createMockHNServer(t)
	defer hnServer.Close()

	reg := collector.NewRegistry()
	hnClient := hackernews.NewClientWithBaseURL(hnServer.URL)
	reg.Register(hackernews.NewWithClient("top", hnClient))

	tmpDir, _ := os.MkdirTemp("", "techpulse-unknown-*")
	defer os.RemoveAll(tmpDir)

	storeCfg := storage.DefaultConfig()
	storeCfg.BaseDir = tmpDir
	tp := &TechPulse{
		config:     Config{Limit: 10, Timeout: 30, Sources: []string{"unknown", "hackernews_top"}},
		registry:   reg,
		pipeline:   filter.DefaultPipeline(),
		summarizer: summarizer.NewBasicSummarizer(),
		storage:    storage.NewMarkdownStorage(storeCfg),
		log:        logger.NewNopLogger(),
	}

	ctx := context.Background()
	if err := tp.Run(ctx); err != nil {
		t.Fatalf("Run() should succeed with partial valid sources: %v", err)
	}
}
