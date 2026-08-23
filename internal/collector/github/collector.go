package github

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/majiayu000/techpulse/internal/collector"
	"github.com/majiayu000/techpulse/internal/httpclient"
)

const defaultBaseURL = "https://github.com"

// minTrendingPageSize is the minimum body size (in bytes) at which a
// zero-repo parse result is treated as a markup change rather than a
// legitimately empty trending page.
const minTrendingPageSize = 10 * 1024

// Collector collects trending repositories from GitHub.
type Collector struct {
	httpClient *http.Client
	baseURL    string
	parser     *Parser
	period     string // daily, weekly, monthly
	language   string // programming language filter
}

// New creates a new GitHub Trending collector with default settings.
func New() *Collector {
	return NewWithOptions(DefaultPeriod, DefaultLanguage)
}

// NewWithOptions creates a collector with custom period and language.
func NewWithOptions(period, language string) *Collector {
	return &Collector{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    defaultBaseURL,
		parser:     NewParser(),
		period:     period,
		language:   language,
	}
}

// NewWithBaseURL creates a collector with custom base URL (for testing).
func NewWithBaseURL(baseURL string) *Collector {
	return &Collector{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    baseURL,
		parser:     NewParser(),
		period:     DefaultPeriod,
		language:   DefaultLanguage,
	}
}

// Name returns the collector's unique name.
func (c *Collector) Name() string {
	if c.language != "" {
		return fmt.Sprintf("github_trending_%s_%s", c.period, c.language)
	}
	return fmt.Sprintf("github_trending_%s", c.period)
}

// Validate checks if the collector configuration is valid.
func (c *Collector) Validate() error {
	if !ValidPeriods[c.period] {
		return fmt.Errorf("invalid period: %s", c.period)
	}
	return nil
}

// Collect fetches trending repositories from GitHub.
func (c *Collector) Collect(ctx context.Context, opts collector.Options) ([]collector.Article, error) {
	html, err := c.fetchTrendingPage(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch trending page: %w", err)
	}

	repos, err := c.parser.Parse(html)
	if err != nil {
		return nil, fmt.Errorf("parse trending page: %w", err)
	}

	if len(repos) == 0 && len(html) > minTrendingPageSize {
		return nil, fmt.Errorf("trending page fetched (%d bytes) but 0 repos parsed - markup may have changed", len(html))
	}

	// Apply limit
	limit := opts.Limit
	if limit <= 0 || limit > len(repos) {
		limit = len(repos)
	}
	repos = repos[:limit]

	return c.toArticles(repos), nil
}

// fetchTrendingPage fetches the GitHub trending HTML page.
func (c *Collector) fetchTrendingPage(ctx context.Context) (string, error) {
	u := c.buildURL()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("User-Agent", "TechPulse/1.0")
	req.Header.Set("Accept", "text/html")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	body, err := httpclient.ReadLimited(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	return string(body), nil
}

// buildURL constructs the trending page URL.
func (c *Collector) buildURL() string {
	u := c.baseURL + "/trending"
	if c.language != "" {
		u += "/" + url.PathEscape(c.language)
	}
	u += "?since=" + c.period
	return u
}

// toArticles converts repositories to collector.Article format.
func (c *Collector) toArticles(repos []Repository) []collector.Article {
	articles := make([]collector.Article, len(repos))
	now := time.Now()

	for i, repo := range repos {
		articles[i] = collector.Article{
			ID:       fmt.Sprintf("gh_%s", sanitizeID(repo.Name)),
			Source:   "github",
			SourceID: repo.Name,
			Title:    repo.Name,
			URL:      repo.URL,
			Content:  repo.Description,
			Author:   extractOwner(repo.Name),
			Score:    repo.Stars,
			Comments: repo.Forks,
			Tags:     buildTags(repo),
			Metadata: map[string]string{
				"language":    repo.Language,
				"stars_today": fmt.Sprintf("%d", repo.StarsToday),
				"period":      c.period,
			},
			PublishedAt: now,
			CollectedAt: now,
		}
	}

	return articles
}
