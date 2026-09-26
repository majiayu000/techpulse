package httpclient

import (
	"context"
	"math"
	"sync"
	"testing"
	"time"
)

func TestDefaultRateLimitConfig(t *testing.T) {
	cfg := DefaultRateLimitConfig()

	if cfg.RequestsPerSecond != 2.0 {
		t.Errorf("RequestsPerSecond = %v, want 2.0", cfg.RequestsPerSecond)
	}
	if cfg.BurstSize != 5 {
		t.Errorf("BurstSize = %d, want 5", cfg.BurstSize)
	}
	if cfg.DefaultDelay != 200*time.Millisecond {
		t.Errorf("DefaultDelay = %v, want 200ms", cfg.DefaultDelay)
	}
}

func TestNewRateLimiter(t *testing.T) {
	cfg := DefaultRateLimitConfig()
	rl := NewRateLimiter(cfg)

	if rl == nil {
		t.Fatal("NewRateLimiter returned nil")
	}
	if rl.limiters == nil {
		t.Error("limiters map is nil")
	}
}
func TestLimiterKey(t *testing.T) {
	tests := []struct {
		url string
		key string
		ok  bool
	}{
		{"https://example.com/path", "https://example.com", true},
		{"https://api.github.com/repos", "https://api.github.com", true},
		{"http://localhost:8080/test", "http://localhost:8080", true},
		{"https://example.com:8443/a", "https://example.com:8443", true},
		{"ftp://files.example.com/x", "ftp://files.example.com", true},
		{"http://example.com/path", "http://example.com", true},
		{"invalid-url", "", false},
		{"mailto:user@example.com", "", false},
		{"/relative/path", "", false},
		{"::bad::", "", false},
	}

	for _, tt := range tests {
		key, ok := limiterKey(tt.url)
		if ok != tt.ok {
			t.Errorf("limiterKey(%q) ok = %v, want %v", tt.url, ok, tt.ok)
		}
		if key != tt.key {
			t.Errorf("limiterKey(%q) = %q, want %q", tt.url, key, tt.key)
		}
	}
}

func TestNewRateLimiterClampsInvalidConfig(t *testing.T) {
	tests := []struct {
		name      string
		cfg       RateLimitConfig
		wantRate  float64
		wantBurst int
	}{
		{
			name:      "zero RPS falls back to DefaultDelay pacing",
			cfg:       RateLimitConfig{RequestsPerSecond: 0, BurstSize: 5, DefaultDelay: 200 * time.Millisecond},
			wantRate:  5.0,
			wantBurst: 5,
		},
		{
			name:      "negative RPS falls back to DefaultDelay pacing",
			cfg:       RateLimitConfig{RequestsPerSecond: -3, BurstSize: 2, DefaultDelay: 500 * time.Millisecond},
			wantRate:  2.0,
			wantBurst: 2,
		},
		{
			name:      "no usable rate falls back to 1 rps",
			cfg:       RateLimitConfig{RequestsPerSecond: 0, BurstSize: 4, DefaultDelay: 0},
			wantRate:  1.0,
			wantBurst: 4,
		},
		{
			name:      "valid config kept as-is",
			cfg:       RateLimitConfig{RequestsPerSecond: 7.5, BurstSize: 3, DefaultDelay: 0},
			wantRate:  7.5,
			wantBurst: 3,
		},
		{
			name:      "burst below 1 clamped to 1",
			cfg:       RateLimitConfig{RequestsPerSecond: 2, BurstSize: 0, DefaultDelay: 0},
			wantRate:  2.0,
			wantBurst: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rl := NewRateLimiter(tt.cfg)
			if math.Abs(rl.config.RequestsPerSecond-tt.wantRate) > 1e-9 {
				t.Errorf("RequestsPerSecond = %v, want %v", rl.config.RequestsPerSecond, tt.wantRate)
			}
			if rl.config.BurstSize != tt.wantBurst {
				t.Errorf("BurstSize = %d, want %d", rl.config.BurstSize, tt.wantBurst)
			}
		})
	}
}

func TestRateLimiterFallbackPacing(t *testing.T) {
	// A non-positive rate must not spin or hang: the fallback paces at one
	// request per DefaultDelay.
	rl := NewRateLimiter(RateLimitConfig{RequestsPerSecond: -1, BurstSize: 1, DefaultDelay: 50 * time.Millisecond})
	ctx := context.Background()

	if err := rl.Wait(ctx, "https://example.com/a"); err != nil {
		t.Fatalf("First Wait() error = %v", err)
	}

	start := time.Now()
	if err := rl.Wait(ctx, "https://example.com/a"); err != nil {
		t.Fatalf("Second Wait() error = %v", err)
	}
	elapsed := time.Since(start)

	if elapsed < 40*time.Millisecond {
		t.Errorf("Throttled request took %v, expected >= 40ms (paced by DefaultDelay)", elapsed)
	}
	if elapsed > 2*time.Second {
		t.Errorf("Throttled request took %v, expected < 2s (must not hang)", elapsed)
	}
}

func TestRateLimiterBurst(t *testing.T) {
	cfg := RateLimitConfig{
		RequestsPerSecond: 1.0,
		BurstSize:         3,
		DefaultDelay:      100 * time.Millisecond,
	}
	rl := NewRateLimiter(cfg)
	ctx := context.Background()

	// First 3 requests should be immediate (burst)
	start := time.Now()
	for i := 0; i < 3; i++ {
		if err := rl.Wait(ctx, "https://example.com/test"); err != nil {
			t.Fatalf("Wait() error = %v", err)
		}
	}
	elapsed := time.Since(start)

	// Burst should complete quickly (under 100ms)
	if elapsed > 100*time.Millisecond {
		t.Errorf("Burst took %v, expected < 100ms", elapsed)
	}
}

func TestRateLimiterThrottling(t *testing.T) {
	cfg := RateLimitConfig{
		RequestsPerSecond: 10.0, // 10 per second = 100ms per request
		BurstSize:         1,    // No burst
		DefaultDelay:      50 * time.Millisecond,
	}
	rl := NewRateLimiter(cfg)
	ctx := context.Background()

	// First request is immediate
	if err := rl.Wait(ctx, "https://example.com/test"); err != nil {
		t.Fatalf("First Wait() error = %v", err)
	}

	// Second request should wait ~100ms
	start := time.Now()
	if err := rl.Wait(ctx, "https://example.com/test"); err != nil {
		t.Fatalf("Second Wait() error = %v", err)
	}
	elapsed := time.Since(start)

	// Should take at least 80ms (with some margin)
	if elapsed < 80*time.Millisecond {
		t.Errorf("Throttled request took %v, expected >= 80ms", elapsed)
	}
}

func TestRateLimiterPerDomain(t *testing.T) {
	cfg := RateLimitConfig{
		RequestsPerSecond: 1.0,
		BurstSize:         1,
		DefaultDelay:      100 * time.Millisecond,
	}
	rl := NewRateLimiter(cfg)
	ctx := context.Background()

	// Request to domain 1
	if err := rl.Wait(ctx, "https://domain1.com/test"); err != nil {
		t.Fatalf("Wait domain1 error = %v", err)
	}

	// Request to domain 2 should be immediate (different domain)
	start := time.Now()
	if err := rl.Wait(ctx, "https://domain2.com/test"); err != nil {
		t.Fatalf("Wait domain2 error = %v", err)
	}
	elapsed := time.Since(start)

	// Different domain should be immediate
	if elapsed > 50*time.Millisecond {
		t.Errorf("Different domain took %v, expected < 50ms", elapsed)
	}
}

func TestRateLimiterContextCancel(t *testing.T) {
	cfg := RateLimitConfig{
		RequestsPerSecond: 0.1, // Very slow
		BurstSize:         1,
		DefaultDelay:      100 * time.Millisecond,
	}
	rl := NewRateLimiter(cfg)

	// First request exhausts the burst
	ctx := context.Background()
	if err := rl.Wait(ctx, "https://example.com/test"); err != nil {
		t.Fatalf("First Wait() error = %v", err)
	}

	// Second request with short timeout
	ctx2, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := rl.Wait(ctx2, "https://example.com/test")
	if err == nil {
		t.Error("Expected context deadline error, got nil")
	}
}

func TestRateLimiterReset(t *testing.T) {
	cfg := DefaultRateLimitConfig()
	rl := NewRateLimiter(cfg)
	ctx := context.Background()

	// Add some limiters
	_ = rl.Wait(ctx, "https://domain1.com/test")
	_ = rl.Wait(ctx, "https://domain2.com/test")

	if len(rl.limiters) != 2 {
		t.Errorf("Expected 2 limiters, got %d", len(rl.limiters))
	}

	rl.Reset()

	if len(rl.limiters) != 0 {
		t.Errorf("After Reset, expected 0 limiters, got %d", len(rl.limiters))
	}
}

func TestRateLimiterConcurrent(t *testing.T) {
	cfg := RateLimitConfig{
		RequestsPerSecond: 100.0, // High rate for test
		BurstSize:         10,
		DefaultDelay:      10 * time.Millisecond,
	}
	rl := NewRateLimiter(cfg)
	ctx := context.Background()

	var wg sync.WaitGroup
	errCh := make(chan error, 20)

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := rl.Wait(ctx, "https://example.com/test"); err != nil {
				errCh <- err
			}
		}()
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("Concurrent Wait() error = %v", err)
	}
}
