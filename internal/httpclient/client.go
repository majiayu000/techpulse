package httpclient

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"syscall"
	"time"
)

// Client is an HTTP client with retry and rate limiting support.
type Client struct {
	httpClient  *http.Client
	retryConfig RetryConfig
	rateLimiter *RateLimiter
	userAgent   string
}

// Option configures the client.
type Option func(*Client)

// WithTimeout sets the HTTP client timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) {
		c.httpClient.Timeout = d
	}
}

// WithRetryConfig sets the retry configuration.
func WithRetryConfig(cfg RetryConfig) Option {
	return func(c *Client) {
		c.retryConfig = cfg
	}
}

// WithUserAgent sets the User-Agent header.
func WithUserAgent(ua string) Option {
	return func(c *Client) {
		c.userAgent = ua
	}
}

// WithRateLimiter sets a custom rate limiter.
func WithRateLimiter(rl *RateLimiter) Option {
	return func(c *Client) {
		c.rateLimiter = rl
	}
}

// WithRateLimitConfig sets rate limiting configuration.
func WithRateLimitConfig(cfg RateLimitConfig) Option {
	return func(c *Client) {
		c.rateLimiter = NewRateLimiter(cfg)
	}
}

// New creates a new HTTP client with the given options.
func New(opts ...Option) *Client {
	c := &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		retryConfig: DefaultRetryConfig(),
		rateLimiter: NewRateLimiter(DefaultRateLimitConfig()),
		userAgent:   "TechPulse/1.0",
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Do executes an HTTP request with retry support.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	return c.DoWithRetry(req.Context(), req)
}

// DoWithRetry executes an HTTP request with retry and rate limiting support.
func (c *Client) DoWithRetry(ctx context.Context, req *http.Request) (*http.Response, error) {
	if c.userAgent != "" && req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", c.userAgent)
	}

	var lastErr error
	for attempt := 0; attempt <= c.retryConfig.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := c.retryConfig.CalculateDelay(attempt - 1)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		// Apply rate limiting before each request
		if c.rateLimiter != nil {
			if err := c.rateLimiter.Wait(ctx, req.URL.String()); err != nil {
				return nil, fmt.Errorf("rate limit wait: %w", err)
			}
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			if !c.isRetryableError(err) {
				return nil, err
			}
			continue
		}

		// Check if we should retry based on status code
		if c.retryConfig.ShouldRetry(resp.StatusCode) && attempt < c.retryConfig.MaxRetries {
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("retryable status: %d", resp.StatusCode)
			continue
		}

		return resp, nil
	}

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

// Get performs a GET request to the specified URL.
func (c *Client) Get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	return c.DoWithRetry(ctx, req)
}

// GetBody performs a GET request and returns the response body.
func (c *Client) GetBody(ctx context.Context, url string) ([]byte, error) {
	resp, err := c.Get(ctx, url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// isRetryableError reports whether err should trigger another request attempt.
// Only timeouts and temporary/network-transient failures are retryable.
// context.Canceled, TLS/x509 certificate errors, and other permanent failures are not.
func (c *Client) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Unwrap common HTTP client wrappers so classification sees the root cause.
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		err = urlErr.Err
	}

	if errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	// Permanent TLS / certificate failures must not be retried.
	if isPermanentTLSError(err) {
		return false
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return dnsErr.Temporary() || dnsErr.Timeout()
	}

	if isTransientSyscallError(err) {
		return true
	}

	return false
}

func isPermanentTLSError(err error) bool {
	var (
		unknownAuth x509.UnknownAuthorityError
		hostnameErr x509.HostnameError
		certInvalid x509.CertificateInvalidError
		systemRoots x509.SystemRootsError
		certVerify  *tls.CertificateVerificationError
	)
	switch {
	case errors.As(err, &unknownAuth),
		errors.As(err, &hostnameErr),
		errors.As(err, &certInvalid),
		errors.As(err, &systemRoots),
		errors.As(err, &certVerify):
		return true
	default:
		return false
	}
}

func isTransientSyscallError(err error) bool {
	var errno syscall.Errno
	switch {
	case errors.As(err, &errno):
		return isRetryableErrno(errno)
	}

	var sysErr *os.SyscallError
	if errors.As(err, &sysErr) {
		if errno, ok := sysErr.Err.(syscall.Errno); ok {
			return isRetryableErrno(errno)
		}
	}

	var opErr *net.OpError
	if errors.As(err, &opErr) {
		if errors.As(opErr.Err, &errno) {
			return isRetryableErrno(errno)
		}
		if sysErr, ok := opErr.Err.(*os.SyscallError); ok {
			if errno, ok := sysErr.Err.(syscall.Errno); ok {
				return isRetryableErrno(errno)
			}
		}
	}

	return false
}

func isRetryableErrno(errno syscall.Errno) bool {
	switch errno {
	case syscall.ECONNRESET, syscall.ECONNREFUSED, syscall.ECONNABORTED,
		syscall.EPIPE, syscall.ETIMEDOUT, syscall.EHOSTUNREACH, syscall.ENETUNREACH:
		return true
	default:
		return false
	}
}
