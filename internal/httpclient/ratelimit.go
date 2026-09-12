// Package httpclient provides HTTP client with retry and rate limiting support.
package httpclient

import (
	"context"
	"net/url"
	"sync"
	"time"

	"github.com/majiayu000/techpulse/internal/logger"
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
	// Values <= 0 are invalid (a non-positive rate can never refill the
	// token bucket); NewRateLimiter replaces them with fallback pacing
	// derived from DefaultDelay instead of rejecting them.
	RequestsPerSecond float64
	// BurstSize is the maximum number of requests allowed in a burst.
	// Values < 1 are treated as 1 by NewRateLimiter.
	BurstSize int
	// DefaultDelay is the fallback pacing between requests to the same
	// domain: when RequestsPerSecond is not positive, the effective rate
	// becomes one request per DefaultDelay. It is not applied on top of a
	// valid RequestsPerSecond.
	DefaultDelay time.Duration
}

// minRequestsPerSecond is the pacing floor used when neither
// RequestsPerSecond nor DefaultDelay yields a usable rate.
const minRequestsPerSecond = 1.0

// DefaultRateLimitConfig returns sensible defaults for rate limiting.
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		RequestsPerSecond: 2.0, // 2 requests per second per domain
		BurstSize:         5,   // Allow bursts of 5 requests
		DefaultDelay:      200 * time.Millisecond,
	}
}

// NewRateLimiter creates a new rate limiter with the given configuration.
//
// Invalid configuration is normalized instead of rejected (this constructor
// cannot return an error): a non-positive RequestsPerSecond would leave the
// token bucket permanently empty and block Wait forever, so it is replaced by
// one request per DefaultDelay — or 1 request/second if DefaultDelay is also
// unset — and BurstSize below 1 is raised to 1. Both normalizations are
// announced through the package logger so they are visible to the user.
func NewRateLimiter(cfg RateLimitConfig) *RateLimiter {
	rate := cfg.RequestsPerSecond
	if rate <= 0 {
		if cfg.DefaultDelay > 0 {
			rate = 1.0 / cfg.DefaultDelay.Seconds()
		} else {
			rate = minRequestsPerSecond
		}
		logger.Warn("httpclient: non-positive RequestsPerSecond, using DefaultDelay-based pacing",
			logger.F("requested_rps", cfg.RequestsPerSecond),
			logger.F("effective_rps", rate))
	}

	burst := cfg.BurstSize
	if burst < 1 {
		logger.Warn("httpclient: BurstSize below 1, clamping to 1",
			logger.F("requested_burst", burst))
		burst = 1
	}

	return &RateLimiter{
		limiters: make(map[string]*domainLimiter),
		config: RateLimitConfig{
			RequestsPerSecond: rate,
			BurstSize:         burst,
			DefaultDelay:      cfg.DefaultDelay,
		},
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
//
// Limiting is keyed by scheme+host. URLs without a parsable host (relative
// references, "mailto:..."-style opaque URLs) cannot be attributed to a
// domain; rate limiting here is best-effort, so such URLs are skipped with a
// warning log rather than failing the request.
func (rl *RateLimiter) Wait(ctx context.Context, rawURL string) error {
	key, ok := limiterKey(rawURL)
	if !ok {
		logger.Warn("httpclient: skipping rate limit for URL without host",
			logger.F("url", rawURL))
		return nil
	}
	return rl.getLimiter(key).wait(ctx)
}

// limiterKey maps a raw URL to its rate limit bucket ("scheme://host").
// ok is false when the URL cannot be parsed or has no host — such requests
// cannot be attributed to a domain and are not rate limited.
func limiterKey(rawURL string) (string, bool) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return "", false
	}
	return parsed.Scheme + "://" + parsed.Host, true
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

// Reset clears all rate limiting state.
func (rl *RateLimiter) Reset() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.limiters = make(map[string]*domainLimiter)
}
