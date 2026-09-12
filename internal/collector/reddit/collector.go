package reddit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/majiayu000/techpulse/internal/collector"
	"github.com/majiayu000/techpulse/internal/logger"
)

// Collector collects posts from Reddit.
type Collector struct {
	httpClient *http.Client
	baseURL    string
	subreddits []Subreddit
	sortType   SortType
}

// New creates a new Reddit collector with default subreddits.
func New() *Collector {
	return NewWithSubreddits(DefaultSubreddits)
}

// NewWithSubreddits creates a collector with custom subreddits.
func NewWithSubreddits(subreddits []Subreddit) *Collector {
	return &Collector{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    BaseURL,
		subreddits: subreddits,
		sortType:   DefaultSortType,
	}
}

// NewWithBaseURL creates a collector with custom base URL (for testing).
func NewWithBaseURL(baseURL string, subreddits []Subreddit) *Collector {
	return &Collector{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    baseURL,
		subreddits: subreddits,
		sortType:   DefaultSortType,
	}
}

// SetHTTPTimeout sets the per-request timeout on the underlying HTTP client.
func (c *Collector) SetHTTPTimeout(d time.Duration) {
	if d > 0 {
		c.httpClient.Timeout = d
	}
}

// Name returns the collector's unique name.
func (c *Collector) Name() string {
	return "reddit"
}

// Validate checks if the collector configuration is valid.
func (c *Collector) Validate() error {
	if len(c.subreddits) == 0 {
		return fmt.Errorf("no subreddits configured")
	}
	if !ValidSortTypes[c.sortType] {
		return fmt.Errorf("invalid sort type: %s", c.sortType)
	}
	return nil
}

// Collect fetches posts from all configured subreddits.
//
// A subreddit that fails to fetch is skipped; if at least one subreddit
// succeeds the collected articles are returned and every failed subreddit is
// logged as a warning. If every configured subreddit fails, Collect returns
// an error naming them instead of silently reporting no results.
func (c *Collector) Collect(ctx context.Context, opts collector.Options) ([]collector.Article, error) {
	var (
		groups    [][]collector.Article // per-subreddit articles, in config order
		fetchErrs []error
	)

	for _, sub := range c.subreddits {
		posts, err := c.fetchSubreddit(ctx, sub)
		if err != nil {
			fetchErrs = append(fetchErrs, fmt.Errorf("subreddit %s: %w", sub.Name, err))
			continue
		}

		// Apply time filter
		if !opts.Since.IsZero() {
			posts = filterByTime(posts, opts.Since)
		}

		groups = append(groups, c.toArticles(posts, sub))
	}

	if len(fetchErrs) > 0 {
		if len(groups) == 0 {
			return nil, fmt.Errorf("reddit: all %d subreddits failed: %w",
				len(fetchErrs), errors.Join(fetchErrs...))
		}
		logger.Warn("some reddit subreddits failed to fetch",
			logger.F("failed", len(fetchErrs)),
			logger.F("errors", errors.Join(fetchErrs...).Error()))
	}

	articles := interleaveArticles(groups)

	// Apply limit across subreddits (interleaving keeps every subreddit represented)
	if opts.Limit > 0 && len(articles) > opts.Limit {
		articles = articles[:opts.Limit]
	}

	return articles, nil
}

// interleaveArticles merges per-subreddit article groups round-robin so a
// later-applied limit trims each subreddit instead of dropping entire ones.
func interleaveArticles(groups [][]collector.Article) []collector.Article {
	total := 0
	maxLen := 0
	for _, g := range groups {
		total += len(g)
		if len(g) > maxLen {
			maxLen = len(g)
		}
	}
	if total == 0 {
		return nil
	}

	out := make([]collector.Article, 0, total)
	for i := 0; i < maxLen; i++ {
		for _, g := range groups {
			if i < len(g) {
				out = append(out, g[i])
			}
		}
	}
	return out
}

// fetchSubreddit fetches posts from a single subreddit.
func (c *Collector) fetchSubreddit(ctx context.Context, sub Subreddit) ([]Post, error) {
	url := fmt.Sprintf("%s/r/%s/%s.json?limit=50", c.baseURL, sub.Name, c.sortType)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Reddit requires a User-Agent
	req.Header.Set("User-Agent", "TechPulse/1.0 (https://github.com/techpulse)")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var listing ListingResponse
	if err := json.NewDecoder(resp.Body).Decode(&listing); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	posts := make([]Post, len(listing.Data.Children))
	for i, child := range listing.Data.Children {
		posts[i] = child.Data
	}

	return posts, nil
}

// filterByTime filters posts published after the given time.
func filterByTime(posts []Post, since time.Time) []Post {
	var filtered []Post
	for _, p := range posts {
		if p.CreatedTime().After(since) {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

// toArticles converts posts to collector.Article format.
func (c *Collector) toArticles(posts []Post, sub Subreddit) []collector.Article {
	articles := make([]collector.Article, len(posts))
	now := time.Now()

	for i, p := range posts {
		// Use external URL for link posts, permalink for self posts
		url := p.URL
		if p.IsSelf || url == "" {
			url = fmt.Sprintf("https://www.reddit.com%s", p.Permalink)
		}

		articles[i] = collector.Article{
			ID:          fmt.Sprintf("reddit_%s", p.ID),
			Source:      "reddit",
			SourceID:    p.ID,
			Title:       p.Title,
			URL:         url,
			Content:     truncateContent(p.Selftext, 500),
			Author:      p.Author,
			Score:       p.Score,
			Comments:    p.NumComments,
			Tags:        buildTags(p, sub),
			Metadata:    buildMetadata(p, sub),
			PublishedAt: p.CreatedTime(),
			CollectedAt: now,
		}
	}

	return articles
}

// truncateContent truncates content to at most maxLen runes, appending an
// ellipsis. It never splits a multi-byte UTF-8 character.
func truncateContent(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

// buildTags creates tags from post data.
func buildTags(p Post, sub Subreddit) []string {
	tags := []string{strings.ToLower(p.Subreddit)}
	if p.Flair != "" {
		tags = append(tags, strings.ToLower(p.Flair))
	}
	if sub.Category != "" {
		tags = append(tags, sub.Category)
	}
	return tags
}

// buildMetadata creates metadata for a post.
func buildMetadata(p Post, sub Subreddit) map[string]string {
	meta := map[string]string{
		"subreddit": p.Subreddit,
		"domain":    p.Domain,
		"permalink": fmt.Sprintf("https://www.reddit.com%s", p.Permalink),
		"category":  sub.Category,
	}
	if p.IsSelf {
		meta["is_self"] = "true"
	}
	return meta
}
