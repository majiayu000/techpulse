package hackernews

import (
	"context"
	"fmt"
	"time"

	"github.com/majiayu000/techpulse/internal/collector"
)

// Collector collects articles from Hacker News.
type Collector struct {
	client   *Client
	category string
}

// New creates a new HN collector for the specified category.
func New(category string) *Collector {
	return &Collector{
		client:   NewClient(),
		category: category,
	}
}

// NewWithClient creates a collector with a custom client (for testing).
func NewWithClient(category string, client *Client) *Collector {
	return &Collector{
		client:   client,
		category: category,
	}
}

// Name returns the collector's unique name.
func (c *Collector) Name() string {
	return fmt.Sprintf("hackernews_%s", c.category)
}

// Validate checks if the collector configuration is valid.
func (c *Collector) Validate() error {
	if !ValidCategories[c.category] {
		return fmt.Errorf("invalid category: %s", c.category)
	}
	return nil
}

// Collect fetches articles from Hacker News.
func (c *Collector) Collect(ctx context.Context, opts collector.Options) ([]collector.Article, error) {
	// Fetch story IDs
	ids, err := c.client.FetchStoryIDs(ctx, c.category)
	if err != nil {
		return nil, fmt.Errorf("fetch story IDs: %w", err)
	}

	// Apply limit
	limit := opts.Limit
	if limit <= 0 || limit > len(ids) {
		limit = len(ids)
	}
	if limit > 50 {
		limit = 50 // Cap at 50 to avoid too many requests
	}
	ids = ids[:limit]

	// Fetch items concurrently, failing closed on total failure or context
	// cancellation instead of silently reporting partial data as success.
	res := c.client.fetchItemsConcurrently(ctx, ids, DefaultConcurrency)
	if res.err != nil {
		return nil, fmt.Errorf("fetch items: %w", res.err)
	}
	items := res.items

	// Filter and convert to articles
	articles := make([]collector.Article, 0, len(items))
	for _, item := range items {
		// Skip deleted or dead items
		if item.Deleted || item.Dead {
			continue
		}

		// Apply time filter
		publishedAt := time.Unix(item.Time, 0)
		if !opts.Since.IsZero() && publishedAt.Before(opts.Since) {
			continue
		}

		articles = append(articles, c.toArticle(item))
	}

	return articles, nil
}

func (c *Collector) toArticle(item *Item) collector.Article {
	url := item.URL
	if url == "" {
		// For Ask HN, Show HN, etc. without external URL
		url = fmt.Sprintf("https://news.ycombinator.com/item?id=%d", item.ID)
	}

	return collector.Article{
		ID:          fmt.Sprintf("hn_%d", item.ID),
		Source:      "hackernews",
		SourceID:    fmt.Sprintf("%d", item.ID),
		Title:       item.Title,
		URL:         url,
		Content:     item.Text,
		Author:      item.By,
		Score:       item.Score,
		Comments:    item.Descendants,
		PublishedAt: time.Unix(item.Time, 0),
		CollectedAt: time.Now(),
		Metadata: map[string]string{
			"category": c.category,
			"type":     item.Type,
		},
	}
}
