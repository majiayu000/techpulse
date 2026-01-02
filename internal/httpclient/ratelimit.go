// Package httpclient provides HTTP client with retry and rate limiting support.
package httpclient

import (
	"context"
	"net/url"
	"sync"
	"time"
)

// RateLimiter limits the rate of HTTP requests per domain.
type RateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*domainLimiter
	config   RateLimitConfig
}

// RateLimitConfig configures rate limiting behavior.
type RateLimitConfig struct {
	// RequestsPerSecond is the sustained rate of requests per domain.
	RequestsPerSecond float64
	// BurstSize is the maximum number of requests allowed in a burst.
	BurstSize int
	// DefaultDelay is the minimum delay between requests to the same domain.
	DefaultDelay time.Duration
}

// DefaultRateLimitConfig returns sensible defaults for rate limiting.
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		RequestsPerSecond: 2.0,  // 2 requests per second per domain
		BurstSize:         5,    // Allow bursts of 5 requests
		DefaultDelay:      200 * time.Millisecond,
	}
}

// NewRateLimiter creates a new rate limiter with the given configuration.
func NewRateLimiter(cfg RateLimitConfig) *RateLimiter {
	return &RateLimiter{
		limiters: make(map[string]*domainLimiter),
		config:   cfg,
	}
}

// domainLimiter implements token bucket rate limiting for a single domain.
type domainLimiter struct {
	mu         sync.Mutex
	tokens     float64
	lastUpdate time.Time
	rate       float64 // tokens per second
	burst      int     // max tokens
}

// newDomainLimiter creates a limiter for a specific domain.
func newDomainLimiter(rate float64, burst int) *domainLimiter {
	return &domainLimiter{
		tokens:     float64(burst), // Start with full bucket
		lastUpdate: time.Now(),
		rate:       rate,
		burst:      burst,
	}
}

// Wait blocks until a request is allowed or context is canceled.
func (rl *RateLimiter) Wait(ctx context.Context, rawURL string) error {
	domain, err := extractDomain(rawURL)
	if err != nil {
		return nil // Don't rate limit on invalid URLs
	}

	limiter := rl.getLimiter(domain)
	return limiter.wait(ctx)
}

// getLimiter returns the limiter for the given domain, creating one if needed.
func (rl *RateLimiter) getLimiter(domain string) *domainLimiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if limiter, ok := rl.limiters[domain]; ok {
		return limiter
	}

	limiter := newDomainLimiter(rl.config.RequestsPerSecond, rl.config.BurstSize)
	rl.limiters[domain] = limiter
	return limiter
}

// wait blocks until a token is available or context is canceled.
func (dl *domainLimiter) wait(ctx context.Context) error {
	for {
		dl.mu.Lock()
		dl.refill()

		if dl.tokens >= 1.0 {
			dl.tokens--
			dl.mu.Unlock()
			return nil
		}

		// Calculate how long until we have a token
		waitTime := time.Duration((1.0 - dl.tokens) / dl.rate * float64(time.Second))
		if waitTime < 10*time.Millisecond {
			waitTime = 10 * time.Millisecond
		}
		dl.mu.Unlock()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitTime):
			// Try again
		}
	}
}

// refill adds tokens based on elapsed time.
func (dl *domainLimiter) refill() {
	now := time.Now()
	elapsed := now.Sub(dl.lastUpdate).Seconds()
	dl.lastUpdate = now

	dl.tokens += elapsed * dl.rate
	if dl.tokens > float64(dl.burst) {
		dl.tokens = float64(dl.burst)
	}
}

// extractDomain extracts the domain from a URL.
func extractDomain(rawURL string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	return parsed.Host, nil
}

// Reset clears all rate limiting state.
func (rl *RateLimiter) Reset() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.limiters = make(map[string]*domainLimiter)
}
