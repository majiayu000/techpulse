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
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Client is an HTTP client with retry, rate limiting, and SSRF protections.
type Client struct {
	httpClient   *http.Client
	retryConfig  RetryConfig
	rateLimiter  *RateLimiter
	userAgent    string
	allowPrivate bool
	maxBodyBytes int64
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

// WithAllowPrivateHosts disables SSRF private/link-local blocking.
// Intended for tests that use httptest on loopback. Do not enable in production.
func WithAllowPrivateHosts(allow bool) Option {
	return func(c *Client) {
		c.allowPrivate = allow
	}
}

// WithMaxResponseBytes overrides the GetBody size cap (default 5 MiB).
// Values <= 0 keep the default.
func WithMaxResponseBytes(n int64) Option {
	return func(c *Client) {
		if n > 0 {
			c.maxBodyBytes = n
		}
	}
}

// New creates a new HTTP client with the given options.
// SSRF protections (dial-time IP checks, redirect re-validation, body size
// caps) are enabled by default.
func New(opts ...Option) *Client {
	c := &Client{
		retryConfig:  DefaultRetryConfig(),
		rateLimiter:  NewRateLimiter(DefaultRateLimitConfig()),
		userAgent:    "TechPulse/1.0",
		maxBodyBytes: DefaultMaxResponseBytes,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	c.applyTransport()
	return c
}

func (c *Client) applyTransport() {
	if c.allowPrivate {
		c.httpClient.Transport = http.DefaultTransport
		c.httpClient.CheckRedirect = nil
		return
	}
	c.httpClient.Transport = ssrfTransport()
	c.httpClient.CheckRedirect = checkRedirect
}

// Do executes an HTTP request with retry support.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	return c.DoWithRetry(req.Context(), req)
}

// DoWithRetry executes an HTTP request with retry and rate limiting support.
//
// Only safe GET, HEAD, and OPTIONS requests are retried, and only when their
// bodies can be replayed faithfully. Caller cancellation and deadlines are
// honored immediately. Retry-After overrides the computed backoff (capped at
// RetryConfig.MaxDelay).
func (c *Client) DoWithRetry(ctx context.Context, req *http.Request) (*http.Response, error) {
	if err := c.validateRequestURL(req.URL); err != nil {
		return nil, err
	}

	if c.userAgent != "" && req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", c.userAgent)
	}

	// A regenerated body alone does not make a request safe to repeat: a POST
	// may already have committed a side effect before a retryable response.
	retryable := (req.Method == http.MethodGet || req.Method == http.MethodHead || req.Method == http.MethodOptions) &&
		(req.Body == nil || req.GetBody != nil)

	var lastErr error
	var retryAfter time.Duration // server-suggested delay for the next attempt
	for attempt := 0; attempt <= c.retryConfig.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := retryAfter
			retryAfter = 0
			if delay <= 0 {
				delay = c.retryConfig.CalculateDelay(attempt - 1)
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}

			// Rewind the request body so the retry sends the full payload.
			if req.Body != nil {
				body, err := req.GetBody()
				if err != nil {
					return nil, fmt.Errorf("regenerate request body for retry: %w", err)
				}
				req.Body = body
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
			if !retryable || !c.isRetryableError(err) {
				return nil, err
			}
			continue
		}

		// Check if we should retry based on status code
		if retryable &&
			c.retryConfig.ShouldRetry(resp.StatusCode) &&
			attempt < c.retryConfig.MaxRetries {
			retryAfter = retryAfterDelay(resp.Header, c.retryConfig.MaxDelay)
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("retryable status: %d", resp.StatusCode)
			continue
		}

		return resp, nil
	}

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

// Get performs a GET request to the specified URL.
func (c *Client) Get(ctx context.Context, rawURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	return c.DoWithRetry(ctx, req)
}

// GetBody performs a GET request and returns the response body.
// The body is capped at maxBodyBytes (default 5 MiB).
func (c *Client) GetBody(ctx context.Context, rawURL string) ([]byte, error) {
	resp, err := c.Get(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	limit := c.maxBodyBytes
	if limit <= 0 {
		limit = DefaultMaxResponseBytes
	}
	limited := io.LimitReader(resp.Body, limit+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("%w (%d bytes)", ErrResponseTooLarge, limit)
	}
	return body, nil
}

// MaxBodyBytes bounds collector response bodies read by ReadLimited.
const MaxBodyBytes = 20 << 20 // 20 MB

// ReadLimited reads a collector response but fails if it exceeds MaxBodyBytes.
func ReadLimited(r io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxBodyBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxBodyBytes {
		return nil, fmt.Errorf("response body exceeds %d bytes", MaxBodyBytes)
	}
	return data, nil
}

func (c *Client) validateRequestURL(u *url.URL) error {
	if c.allowPrivate {
		return nil
	}
	return ValidateURL(u)
}

// isRetryableError reports whether err should trigger another request attempt.
// Only timeouts and temporary/network-transient failures are retryable.
// SSRF policy failures, context.Canceled, TLS/x509 certificate errors, and
// other permanent failures are not.
func (c *Client) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// SSRF policy failures must not be retried.
	if errors.Is(err, ErrSSRFBlocked) || errors.Is(err, ErrInvalidScheme) || errors.Is(err, ErrTooManyRedirects) {
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

	// Peer closed an idle persistent connection — commonly transient.
	if errors.Is(err, io.EOF) {
		return true
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
		syscall.EPIPE, syscall.ETIMEDOUT, syscall.EHOSTUNREACH, syscall.ENETUNREACH,
		syscall.ENETDOWN:
		return true
	default:
		return false
	}
}

// retryAfterDelay extracts the delay requested by a Retry-After header
// (seconds or HTTP-date form). It returns 0 when the header is absent,
// malformed, or already past — the caller then falls back to its own
// exponential backoff. The result is capped at maxDelay so a hostile or
// misconfigured server cannot stall the caller indefinitely.
func retryAfterDelay(h http.Header, maxDelay time.Duration) time.Duration {
	v := strings.TrimSpace(h.Get("Retry-After"))
	if v == "" {
		return 0
	}
	var d time.Duration
	if secs, err := strconv.Atoi(v); err == nil && secs > 0 {
		d = time.Duration(secs) * time.Second
	} else if t, err := http.ParseTime(v); err == nil {
		if until := time.Until(t); until > 0 {
			d = until
		}
	}
	switch {
	case d <= 0:
		return 0
	case d > maxDelay:
		return maxDelay
	default:
		return d
	}
}
