// Package collector provides interfaces and types for data collection.
package collector

import (
	"context"
	"time"
)

// Article represents an article collected from any data source.
type Article struct {
	ID          string            `json:"id"`
	Source      string            `json:"source"`
	SourceID    string            `json:"source_id"`
	Title       string            `json:"title"`
	URL         string            `json:"url"`
	Content     string            `json:"content,omitempty"`
	Author      string            `json:"author,omitempty"`
	Score       int               `json:"score"`
	Comments    int               `json:"comments"`
	Tags        []string          `json:"tags,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	PublishedAt time.Time         `json:"published_at"`
	CollectedAt time.Time         `json:"collected_at"`
}

// Collector defines the interface for data source collectors.
type Collector interface {
	// Name returns the collector's unique name.
	Name() string

	// Collect fetches articles from the data source.
	Collect(ctx context.Context, opts Options) ([]Article, error)

	// Validate checks if the collector configuration is valid.
	Validate() error
}

// Options contains options for the Collect operation.
type Options struct {
	Limit    int           // Maximum number of articles to collect
	Since    time.Time     // Collect articles published after this time
	Category string        // Category filter (source-specific)
	Timeout  time.Duration // Request timeout
}

// Result represents the outcome of a collection operation.
type Result struct {
	Source    string        // Collector name
	Articles  []Article     // Collected articles
	Error     error         // Error if collection failed
	Duration  time.Duration // Time taken to collect
	Timestamp time.Time     // When collection completed
}

// DefaultOptions returns sensible default options.
func DefaultOptions() Options {
	return Options{
		Limit:   30,
		Timeout: 30 * time.Second,
	}
}
