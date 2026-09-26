package lobsters

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/majiayu000/techpulse/internal/collector"
	"github.com/majiayu000/techpulse/internal/httpclient"
)

// Collector collects stories from Lobsters.
type Collector struct {
	httpClient *http.Client
	baseURL    string
	feedType   FeedType
	tags       []string // Optional tag filters
}

// New creates a new Lobsters collector with default settings.
func New() *Collector {
	return NewWithOptions(DefaultFeedType, nil)
}

// NewWithOptions creates a collector with custom feed type and tag filters.
func NewWithOptions(feedType FeedType, tags []string) *Collector {
	return &Collector{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    DefaultBaseURL,
		feedType:   feedType,
		tags:       tags,
	}
}

// NewWithBaseURL creates a collector with custom base URL (for testing).
func NewWithBaseURL(baseURL string) *Collector {
	return &Collector{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    baseURL,
		feedType:   DefaultFeedType,
		tags:       nil,
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
	if len(c.tags) > 0 {
		return fmt.Sprintf("lobsters_%s_%s", c.feedType, strings.Join(c.tags, "_"))
	}
	return fmt.Sprintf("lobsters_%s", c.feedType)
}

// Validate checks if the collector configuration is valid.
func (c *Collector) Validate() error {
	if !ValidFeedTypes[c.feedType] {
		return fmt.Errorf("invalid feed type: %s", c.feedType)
	}
	return nil
}

// Collect fetches stories from Lobsters.
func (c *Collector) Collect(ctx context.Context, opts collector.Options) ([]collector.Article, error) {
	stories, err := c.fetchStories(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch stories: %w", err)
	}

	// Apply time filter
	if !opts.Since.IsZero() {
		stories = filterByTime(stories, opts.Since)
	}

	// Apply limit
	limit := opts.Limit
	if limit <= 0 || limit > len(stories) {
		limit = len(stories)
	}
	stories = stories[:limit]

	return c.toArticles(stories), nil
}

// fetchStories fetches stories from the Lobsters JSON API.
func (c *Collector) fetchStories(ctx context.Context) ([]Story, error) {
	url := fmt.Sprintf("%s/%s.json", c.baseURL, c.feedType)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("User-Agent", "TechPulse/1.0")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	body, err := httpclient.ReadLimited(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var stories []Story
	if err := json.Unmarshal(body, &stories); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}

	// Filter by tags if specified
	if len(c.tags) > 0 {
		stories = filterByTags(stories, c.tags)
	}

	return stories, nil
}

// filterByTime filters stories published after the given time.
func filterByTime(stories []Story, since time.Time) []Story {
	var filtered []Story
	for _, s := range stories {
		if s.CreatedAt.After(since) {
			filtered = append(filtered, s)
		}
	}
	return filtered
}

// filterByTags filters stories that have at least one matching tag.
func filterByTags(stories []Story, tags []string) []Story {
	tagSet := make(map[string]bool)
	for _, t := range tags {
		tagSet[strings.ToLower(t)] = true
	}

	var filtered []Story
	for _, s := range stories {
		for _, t := range s.Tags {
			if tagSet[strings.ToLower(t)] {
				filtered = append(filtered, s)
				break
			}
		}
	}
	return filtered
}

// toArticles converts stories to collector.Article format.
func (c *Collector) toArticles(stories []Story) []collector.Article {
	articles := make([]collector.Article, len(stories))
	now := time.Now()

	for i, s := range stories {
		// Use the story URL, or fallback to short_id_url for text posts
		url := s.URL
		if url == "" {
			url = s.ShortIDURL
		}

		articles[i] = collector.Article{
			ID:          fmt.Sprintf("lobsters_%s", s.ShortID),
			Source:      "lobsters",
			SourceID:    s.ShortID,
			Title:       s.Title,
			URL:         url,
			Content:     s.DescriptionPlain,
			Author:      s.SubmitterUser,
			Score:       s.Score,
			Comments:    s.CommentCount,
			Tags:        s.Tags,
			Metadata:    buildMetadata(s),
			PublishedAt: s.CreatedAt,
			CollectedAt: now,
		}
	}

	return articles
}

// buildMetadata creates metadata for a story.
func buildMetadata(s Story) map[string]string {
	meta := map[string]string{
		"comments_url": s.CommentsURL,
	}
	if s.UserIsAuthor {
		meta["user_is_author"] = "true"
	}
	if s.Flags > 0 {
		meta["flags"] = fmt.Sprintf("%d", s.Flags)
	}
	return meta
}
