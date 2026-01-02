// Package extractor provides article content extraction and summarization.
package extractor

import (
	"context"
	"unicode/utf8"

	"github.com/anthropic/autonomous-runner/internal/httpclient"
)

// Extractor extracts summaries from article URLs.
type Extractor struct {
	client *httpclient.Client
	config Config
}

// Config configures the extractor behavior.
type Config struct {
	MaxLength   int  // Maximum summary length in characters
	FetchRemote bool // Whether to fetch remote content
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		MaxLength:   200,
		FetchRemote: true,
	}
}

// New creates a new Extractor.
func New(client *httpclient.Client, cfg Config) *Extractor {
	return &Extractor{
		client: client,
		config: cfg,
	}
}

// NewDefault creates an Extractor with default config.
func NewDefault(client *httpclient.Client) *Extractor {
	return New(client, DefaultConfig())
}

// Extract fetches and extracts a summary from the given URL.
func (e *Extractor) Extract(ctx context.Context, url, existingContent string) (string, error) {
	// If we already have content, use that
	if existingContent != "" {
		return e.truncate(cleanText(existingContent)), nil
	}

	// Skip if remote fetching is disabled
	if !e.config.FetchRemote {
		return "", nil
	}

	// Fetch the page content
	body, err := e.client.GetBody(ctx, url)
	if err != nil {
		return "", nil // Return empty on error, don't fail the whole process
	}

	// Extract text from HTML
	text := extractTextFromHTML(string(body))
	return e.truncate(cleanText(text)), nil
}

func (e *Extractor) truncate(s string) string {
	if utf8.RuneCountInString(s) <= e.config.MaxLength {
		return s
	}
	runes := []rune(s)
	return string(runes[:e.config.MaxLength]) + "..."
}
