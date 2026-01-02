package httpclient

import (
	"net/http"
	"testing"
	"time"
)

func TestDefaultRetryConfig(t *testing.T) {
	cfg := DefaultRetryConfig()

	if cfg.MaxRetries != 3 {
		t.Errorf("expected MaxRetries=3, got %d", cfg.MaxRetries)
	}
	if cfg.InitialDelay != 500*time.Millisecond {
		t.Errorf("expected InitialDelay=500ms, got %v", cfg.InitialDelay)
	}
	if cfg.MaxDelay != 10*time.Second {
		t.Errorf("expected MaxDelay=10s, got %v", cfg.MaxDelay)
	}
	if cfg.Multiplier != 2.0 {
		t.Errorf("expected Multiplier=2.0, got %f", cfg.Multiplier)
	}
}

func TestNoRetryConfig(t *testing.T) {
	cfg := NoRetryConfig()

	if cfg.MaxRetries != 0 {
		t.Errorf("expected MaxRetries=0, got %d", cfg.MaxRetries)
	}
}

func TestShouldRetry(t *testing.T) {
	cfg := DefaultRetryConfig()

	tests := []struct {
		status int
		want   bool
	}{
		{http.StatusOK, false},
		{http.StatusNotFound, false},
		{http.StatusTooManyRequests, true},
		{http.StatusInternalServerError, true},
		{http.StatusBadGateway, true},
		{http.StatusServiceUnavailable, true},
		{http.StatusGatewayTimeout, true},
	}

	for _, tt := range tests {
		got := cfg.ShouldRetry(tt.status)
		if got != tt.want {
			t.Errorf("ShouldRetry(%d) = %v, want %v", tt.status, got, tt.want)
		}
	}
}

func TestCalculateDelay(t *testing.T) {
	cfg := RetryConfig{
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		Multiplier:   2.0,
		Jitter:       0, // No jitter for predictable tests
	}

	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{0, 100 * time.Millisecond},
		{1, 200 * time.Millisecond},
		{2, 400 * time.Millisecond},
		{3, 800 * time.Millisecond},
		{4, 1 * time.Second}, // Capped at MaxDelay
		{5, 1 * time.Second}, // Still capped
	}

	for _, tt := range tests {
		got := cfg.CalculateDelay(tt.attempt)
		if got != tt.want {
			t.Errorf("CalculateDelay(%d) = %v, want %v", tt.attempt, got, tt.want)
		}
	}
}

func TestCalculateDelayWithJitter(t *testing.T) {
	cfg := RetryConfig{
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		Multiplier:   2.0,
		Jitter:       0.5, // 50% jitter
	}

	// With jitter, delays should be within a range
	for i := 0; i < 10; i++ {
		delay := cfg.CalculateDelay(0)
		minDelay := 50 * time.Millisecond  // 100ms - 50%
		maxDelay := 150 * time.Millisecond // 100ms + 50%

		if delay < minDelay || delay > maxDelay {
			t.Errorf("delay %v outside expected range [%v, %v]", delay, minDelay, maxDelay)
		}
	}
}
