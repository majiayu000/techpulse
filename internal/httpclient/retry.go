// Package httpclient provides HTTP client with retry support.
package httpclient

import (
	"math"
	"math/rand"
	"net/http"
	"time"
)

// RetryConfig defines retry behavior for HTTP requests.
type RetryConfig struct {
	// MaxRetries is the maximum number of retry attempts (0 means no retries).
	MaxRetries int
	// InitialDelay is the delay before the first retry.
	InitialDelay time.Duration
	// MaxDelay is the maximum delay between retries.
	MaxDelay time.Duration
	// Multiplier is the factor to multiply delay by after each retry.
	Multiplier float64
	// Jitter adds randomness to the delay (0.0 to 1.0).
	Jitter float64
	// RetryableStatuses are HTTP status codes that should trigger a retry.
	RetryableStatuses []int
}

// DefaultRetryConfig returns a sensible default retry configuration.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:   3,
		InitialDelay: 500 * time.Millisecond,
		MaxDelay:     10 * time.Second,
		Multiplier:   2.0,
		Jitter:       0.1,
		RetryableStatuses: []int{
			http.StatusTooManyRequests,     // 429
			http.StatusInternalServerError, // 500
			http.StatusBadGateway,          // 502
			http.StatusServiceUnavailable,  // 503
			http.StatusGatewayTimeout,      // 504
		},
	}
}

// NoRetryConfig returns a configuration that disables retries.
func NoRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries: 0,
	}
}

// ShouldRetry checks if the status code should trigger a retry.
func (c RetryConfig) ShouldRetry(statusCode int) bool {
	for _, s := range c.RetryableStatuses {
		if s == statusCode {
			return true
		}
	}
	return false
}

// CalculateDelay returns the delay for the given attempt number.
func (c RetryConfig) CalculateDelay(attempt int) time.Duration {
	if attempt <= 0 {
		return c.InitialDelay
	}

	delay := float64(c.InitialDelay) * math.Pow(c.Multiplier, float64(attempt))

	// Apply jitter
	if c.Jitter > 0 {
		jitterRange := delay * c.Jitter
		delay = delay - jitterRange + (rand.Float64() * 2 * jitterRange)
	}

	// Cap at max delay
	if delay > float64(c.MaxDelay) {
		delay = float64(c.MaxDelay)
	}

	return time.Duration(delay)
}
