package hackernews

import (
	"github.com/majiayu000/techpulse/internal/httpclient"
)

// newFastTestClient returns a client pointed at baseURL whose rate limiter is
// relaxed so tests hitting local httptest servers are not throttled at
// production rates (the default limiter allows only 2 req/s per domain).
// Retry is disabled, matching NewClientWithBaseURL.
func newFastTestClient(baseURL string) *Client {
	return &Client{
		httpClient: httpclient.New(
			httpclient.WithRetryConfig(httpclient.NoRetryConfig()),
			httpclient.WithRateLimitConfig(httpclient.RateLimitConfig{
				RequestsPerSecond: 10000,
				BurstSize:         1000,
			}),
		),
		baseURL: baseURL,
	}
}

// parseItemIDFromPath parses item ID from URL path like /item/123.json
func parseItemIDFromPath(path string) int {
	var id int
	prefix := "/item/"
	if len(path) > len(prefix) {
		for i := len(prefix); i < len(path); i++ {
			c := path[i]
			if c >= '0' && c <= '9' {
				id = id*10 + int(c-'0')
			} else {
				break
			}
		}
	}
	return id
}
