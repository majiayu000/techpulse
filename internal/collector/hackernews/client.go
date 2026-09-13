package hackernews

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/anthropic/autonomous-runner/internal/httpclient"
)

const defaultBaseURL = "https://hacker-news.firebaseio.com/v0"

// Client handles HTTP requests to the Hacker News API.
type Client struct {
	httpClient *httpclient.Client
	baseURL    string
}

// NewClient creates a new HN API client with retry support.
func NewClient() *Client {
	return &Client{
		httpClient: httpclient.New(),
		baseURL:    defaultBaseURL,
	}
}

// NewClientWithBaseURL creates a client with a custom base URL (for testing).
func NewClientWithBaseURL(baseURL string) *Client {
	return &Client{
		httpClient: httpclient.New(
			httpclient.WithRetryConfig(httpclient.NoRetryConfig()),
			httpclient.WithAllowPrivateHosts(true),
		),
		baseURL: baseURL,
	}
}

// FetchStoryIDs fetches story IDs for a given category.
func (c *Client) FetchStoryIDs(ctx context.Context, category string) ([]int, error) {
	url := fmt.Sprintf("%s/%sstories.json", c.baseURL, category)

	resp, err := c.httpClient.Get(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var ids []int
	if err := json.NewDecoder(resp.Body).Decode(&ids); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return ids, nil
}

// FetchItem fetches a single item by ID.
func (c *Client) FetchItem(ctx context.Context, id int) (*Item, error) {
	url := fmt.Sprintf("%s/item/%d.json", c.baseURL, id)

	resp, err := c.httpClient.Get(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var item Item
	if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &item, nil
}
