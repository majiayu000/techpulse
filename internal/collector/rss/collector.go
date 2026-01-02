package rss

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/anthropic/autonomous-runner/internal/collector"
	"github.com/anthropic/autonomous-runner/internal/httpclient"
)

// Collector collects articles from RSS feeds.
type Collector struct {
	sources    []Source
	httpClient *httpclient.Client
}

// New creates a new RSS collector with the specified sources.
func New(sources []Source) *Collector {
	return &Collector{
		sources:    sources,
		httpClient: httpclient.New(),
	}
}

// NewWithDefaults creates an RSS collector with default sources.
func NewWithDefaults() *Collector {
	return New(DefaultSources)
}

// Name returns the collector's unique name.
func (c *Collector) Name() string {
	return "rss"
}

// Validate checks if the collector configuration is valid.
func (c *Collector) Validate() error {
	if len(c.sources) == 0 {
		return fmt.Errorf("no RSS sources configured")
	}
	return nil
}

// Collect fetches articles from all configured RSS feeds.
func (c *Collector) Collect(ctx context.Context, opts collector.Options) ([]collector.Article, error) {
	var allArticles []collector.Article

	for _, source := range c.sources {
		articles, err := c.fetchFeed(ctx, source, opts)
		if err != nil {
			// Log error but continue with other sources
			continue
		}
		allArticles = append(allArticles, articles...)
	}

	// Apply limit
	if opts.Limit > 0 && len(allArticles) > opts.Limit {
		allArticles = allArticles[:opts.Limit]
	}

	return allArticles, nil
}

func (c *Collector) fetchFeed(ctx context.Context, source Source, opts collector.Options) ([]collector.Article, error) {
	resp, err := c.httpClient.Get(ctx, source.URL)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var feed Feed
	if err := xml.NewDecoder(resp.Body).Decode(&feed); err != nil {
		return nil, fmt.Errorf("decode RSS: %w", err)
	}

	articles := make([]collector.Article, 0, len(feed.Channel.Items))
	for _, item := range feed.Channel.Items {
		pubTime := c.parseTime(item.PubDate)

		// Apply time filter
		if !opts.Since.IsZero() && pubTime.Before(opts.Since) {
			continue
		}

		articles = append(articles, c.toArticle(item, source, pubTime))
	}

	return articles, nil
}

func (c *Collector) toArticle(item Item, source Source, pubTime time.Time) collector.Article {
	author := item.Author
	if author == "" {
		author = item.Creator
	}

	guid := item.GUID
	if guid == "" {
		guid = item.Link
	}

	return collector.Article{
		ID:          fmt.Sprintf("rss_%s_%s", strings.ToLower(source.Name), hashString(guid)),
		Source:      "rss",
		SourceID:    guid,
		Title:       item.Title,
		URL:         item.Link,
		Content:     item.Description,
		Author:      author,
		Tags:        item.Categories,
		PublishedAt: pubTime,
		CollectedAt: time.Now(),
		Metadata: map[string]string{
			"feed_name": source.Name,
			"feed_url":  source.URL,
		},
	}
}

func (c *Collector) parseTime(s string) time.Time {
	formats := []string{
		time.RFC1123Z,
		time.RFC1123,
		time.RFC822Z,
		time.RFC822,
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02 15:04:05",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t
		}
	}
	return time.Now()
}

func hashString(s string) string {
	h := uint32(0)
	for _, c := range s {
		h = h*31 + uint32(c)
	}
	return fmt.Sprintf("%08x", h)
}
