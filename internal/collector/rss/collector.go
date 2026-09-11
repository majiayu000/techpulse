package rss

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/majiayu000/techpulse/internal/collector"
	"github.com/majiayu000/techpulse/internal/httpclient"
	"github.com/majiayu000/techpulse/internal/logger"
)

// Collector collects articles from RSS and Atom feeds.
type Collector struct {
	sources    []Source
	httpClient *httpclient.Client
	logger     logger.Logger
}

// New creates a new RSS collector with the specified sources.
// Optional httpclient options configure the underlying request client
// (for example per-request timeout).
func New(sources []Source, opts ...httpclient.Option) *Collector {
	return &Collector{
		sources:    sources,
		httpClient: httpclient.New(opts...),
		// logger defaults to nil: per-feed failure warnings go through the
		// package default logger so --quiet / logger.SetDefault are honored.
		// Use WithLogger to override.
	}
}

// WithLogger overrides the logger used to report per-feed failures.
// Pass logger.NewNopLogger() to silence failure reporting.
func (c *Collector) WithLogger(l logger.Logger) *Collector {
	c.logger = l
	return c
}

// NewWithDefaults creates an RSS collector with default sources.
func NewWithDefaults(opts ...httpclient.Option) *Collector {
	return New(DefaultSources, opts...)
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

// Collect fetches articles from all configured RSS/Atom feeds.
//
// A feed that fails to fetch or parse does not abort collection: results
// from healthy feeds are returned and failures are logged. Only when every
// configured feed fails does Collect return an error naming each source.
func (c *Collector) Collect(ctx context.Context, opts collector.Options) ([]collector.Article, error) {
	feeds := make([][]collector.Article, 0, len(c.sources))
	var failures []string

	for _, source := range c.sources {
		articles, err := c.fetchFeed(ctx, source, opts)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", source.Name, err))
			continue
		}
		feeds = append(feeds, articles)
	}

	if len(feeds) == 0 && len(failures) > 0 {
		return nil, fmt.Errorf("all %d RSS feeds failed: %s",
			len(failures), strings.Join(failures, "; "))
	}

	if len(failures) > 0 {
		fields := []logger.Field{
			{Key: "collector", Value: c.Name()},
			{Key: "failed_feeds", Value: strings.Join(failures, "; ")},
		}
		if c.logger != nil {
			c.logger.Warn("some RSS feeds failed; returning partial results", fields...)
		} else {
			logger.Warn("some RSS feeds failed; returning partial results", fields...)
		}
	}

	return applyLimit(feeds, opts.Limit), nil
}

// applyLimit merges per-feed articles and applies the requested limit.
// When truncating, articles are taken round-robin across feeds so that no
// feed is truncated away entirely.
func applyLimit(feeds [][]collector.Article, limit int) []collector.Article {
	total := 0
	for _, f := range feeds {
		total += len(f)
	}

	if limit <= 0 || total <= limit {
		all := make([]collector.Article, 0, total)
		for _, f := range feeds {
			all = append(all, f...)
		}
		return all
	}

	limited := make([]collector.Article, 0, limit)
	for i := 0; len(limited) < limit; i++ {
		took := false
		for _, f := range feeds {
			if i >= len(f) {
				continue
			}
			limited = append(limited, f[i])
			took = true
			if len(limited) == limit {
				break
			}
		}
		if !took {
			break
		}
	}
	return limited
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

	data, err := httpclient.ReadLimited(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read feed body: %w", err)
	}

	items, err := decodeFeed(data, source.URL)
	if err != nil {
		return nil, err
	}

	articles := make([]collector.Article, 0, len(items))
	for _, item := range items {
		pubTime := c.parseTime(item.PubDate)

		// Apply the Since filter only to known publication dates. Entries
		// with unparseable dates are kept with a zero timestamp rather than
		// being silently dropped or stamped with a fabricated time.
		if !opts.Since.IsZero() && !pubTime.IsZero() && pubTime.Before(opts.Since) {
			continue
		}

		articles = append(articles, c.toArticle(item, source, pubTime))
	}

	return articles, nil
}

// decodeFeed decodes an RSS or Atom document into the shared Item shape.
// The format is detected from the document's root element name. feedURL is
// the retrieval URL used to resolve relative Atom entry links.
func decodeFeed(data []byte, feedURL string) ([]Item, error) {
	root, err := rootElement(data)
	if err != nil {
		return nil, fmt.Errorf("inspect feed root element: %w", err)
	}

	switch root {
	case "rss":
		var feed Feed
		if err := xml.Unmarshal(data, &feed); err != nil {
			return nil, fmt.Errorf("decode RSS: %w", err)
		}
		items := make([]Item, 0, len(feed.Channel.Items))
		for _, item := range feed.Channel.Items {
			items = append(items, item)
		}
		return items, nil
	case "feed":
		var feed AtomFeed
		if err := xml.Unmarshal(data, &feed); err != nil {
			return nil, fmt.Errorf("decode Atom: %w", err)
		}
		items := make([]Item, 0, len(feed.Entries))
		for _, entry := range feed.Entries {
			items = append(items, entry.toItem(feedURL, feed.XMLBase))
		}
		return items, nil
	default:
		return nil, fmt.Errorf("unsupported feed format: root element %q", root)
	}
}

// rootElement returns the local name of the document's root XML element.
func rootElement(data []byte) (string, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := dec.Token()
		if err != nil {
			return "", err
		}
		if start, ok := tok.(xml.StartElement); ok {
			return start.Name.Local, nil
		}
	}
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

// parseTime parses a syndication timestamp using the common feed layouts,
// including RFC 3339 with optional fractional seconds. It returns the zero
// time when the value cannot be parsed; substituting the current time would
// fabricate data and defeat Since filtering.
func (c *Collector) parseTime(s string) time.Time {
	formats := []string{
		time.RFC1123Z,
		time.RFC1123,
		time.RFC822Z,
		time.RFC822,
		time.RFC3339Nano,
		"2006-01-02 15:04:05",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

func hashString(s string) string {
	h := uint32(0)
	for _, c := range s {
		h = h*31 + uint32(c)
	}
	return fmt.Sprintf("%08x", h)
}
