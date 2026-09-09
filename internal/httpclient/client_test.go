package httpclient

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	c := New()

	if c.httpClient == nil {
		t.Error("expected httpClient to be set")
	}
	if c.userAgent != "TechPulse/1.0" {
		t.Errorf("expected userAgent=TechPulse/1.0, got %s", c.userAgent)
	}
	if c.retryConfig.MaxRetries != 3 {
		t.Errorf("expected MaxRetries=3, got %d", c.retryConfig.MaxRetries)
	}
}

func TestNewWithOptions(t *testing.T) {
	c := New(
		WithTimeout(5*time.Second),
		WithUserAgent("TestAgent/1.0"),
		WithRetryConfig(NoRetryConfig()),
	)

	if c.httpClient.Timeout != 5*time.Second {
		t.Errorf("expected timeout=5s, got %v", c.httpClient.Timeout)
	}
	if c.userAgent != "TestAgent/1.0" {
		t.Errorf("expected userAgent=TestAgent/1.0, got %s", c.userAgent)
	}
	if c.retryConfig.MaxRetries != 0 {
		t.Errorf("expected MaxRetries=0, got %d", c.retryConfig.MaxRetries)
	}
}

func testClient(opts ...Option) *Client {
	base := []Option{WithAllowPrivateHosts(true)}
	return New(append(base, opts...)...)
}

func TestGet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	c := testClient()
	resp, err := c.Get(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestGetBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello world"))
	}))
	defer server.Close()

	c := testClient()
	body, err := c.GetBody(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(body) != "hello world" {
		t.Errorf("expected 'hello world', got %s", string(body))
	}
}

func TestRetryOnServerError(t *testing.T) {
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&attempts, 1)
		if count < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))
	defer server.Close()

	c := testClient(WithRetryConfig(RetryConfig{
		MaxRetries:        3,
		InitialDelay:      10 * time.Millisecond,
		MaxDelay:          50 * time.Millisecond,
		Multiplier:        2.0,
		Jitter:            0,
		RetryableStatuses: []int{http.StatusServiceUnavailable},
	}))

	body, err := c.GetBody(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
	if string(body) != "success" {
		t.Errorf("expected 'success', got %s", string(body))
	}
}

func TestNoRetryOnNonRetryableStatus(t *testing.T) {
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	c := testClient(WithRetryConfig(RetryConfig{
		MaxRetries:        3,
		InitialDelay:      10 * time.Millisecond,
		MaxDelay:          50 * time.Millisecond,
		Multiplier:        2.0,
		RetryableStatuses: []int{http.StatusServiceUnavailable},
	}))

	_, err := c.GetBody(context.Background(), server.URL)
	if err == nil {
		t.Fatal("expected error for 404")
	}

	if atomic.LoadInt32(&attempts) != 1 {
		t.Errorf("expected 1 attempt (no retry), got %d", attempts)
	}
}

func TestMaxRetriesExceeded(t *testing.T) {
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	c := testClient(WithRetryConfig(RetryConfig{
		MaxRetries:        2,
		InitialDelay:      10 * time.Millisecond,
		MaxDelay:          50 * time.Millisecond,
		Multiplier:        2.0,
		RetryableStatuses: []int{http.StatusServiceUnavailable},
	}))

	_, err := c.GetBody(context.Background(), server.URL)
	if err == nil {
		t.Fatal("expected error after max retries")
	}

	// Initial attempt + 2 retries = 3 total
	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := testClient()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := c.Get(ctx, server.URL)
	if err == nil {
		t.Fatal("expected error due to context cancellation")
	}
}

func TestUserAgentHeader(t *testing.T) {
	var receivedUA string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedUA = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := testClient(WithUserAgent("CustomAgent/2.0"))
	resp, err := c.Get(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()

	if receivedUA != "CustomAgent/2.0" {
		t.Errorf("expected User-Agent 'CustomAgent/2.0', got '%s'", receivedUA)
	}
}

func TestIsRetryableError(t *testing.T) {
	c := New()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "context canceled", err: context.Canceled, want: false},
		{name: "wrapped context canceled", err: fmt.Errorf("get: %w", context.Canceled), want: false},
		{name: "url wrapped context canceled", err: &url.Error{Op: "Get", URL: "https://example.com", Err: context.Canceled}, want: false},
		{name: "context deadline exceeded", err: context.DeadlineExceeded, want: true},
		{name: "url wrapped deadline", err: &url.Error{Op: "Get", URL: "https://example.com", Err: context.DeadlineExceeded}, want: true},
		{name: "x509 unknown authority", err: x509.UnknownAuthorityError{}, want: false},
		{name: "x509 hostname mismatch", err: x509.HostnameError{Host: "example.com", Certificate: &x509.Certificate{}}, want: false},
		{name: "x509 certificate invalid", err: x509.CertificateInvalidError{Reason: x509.Expired, Cert: &x509.Certificate{}}, want: false},
		{name: "tls certificate verification", err: &tls.CertificateVerificationError{Err: x509.UnknownAuthorityError{}}, want: false},
		{name: "url wrapped tls cert error", err: &url.Error{Op: "Get", URL: "https://example.com", Err: x509.UnknownAuthorityError{}}, want: false},
		{name: "temporary dns", err: &net.DNSError{Err: "temporary failure", IsTemporary: true}, want: true},
		{name: "dns timeout", err: &net.DNSError{Err: "i/o timeout", IsTimeout: true}, want: true},
		{name: "permanent dns not found", err: &net.DNSError{Err: "no such host", IsNotFound: true}, want: false},
		{name: "connection reset", err: syscall.ECONNRESET, want: true},
		{name: "connection refused", err: syscall.ECONNREFUSED, want: true},
		{name: "generic permanent error", err: errors.New("something went wrong"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c.isRetryableError(tt.err)
			if got != tt.want {
				t.Fatalf("isRetryableError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestDoWithRetryDoesNotRetryCanceledContext(t *testing.T) {
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := New(WithRetryConfig(RetryConfig{
		MaxRetries:   3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     50 * time.Millisecond,
		Multiplier:   2.0,
	}))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.Get(ctx, server.URL)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if got := atomic.LoadInt32(&attempts); got != 0 {
		t.Fatalf("expected 0 server attempts for canceled context, got %d", got)
	}
}
