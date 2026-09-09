package httpclient

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestIsBlockedIP(t *testing.T) {
	tests := []struct {
		ip      string
		blocked bool
	}{
		{"127.0.0.1", true},
		{"::1", true},
		{"10.0.0.1", true},
		{"172.16.5.1", true},
		{"192.168.1.1", true},
		{"169.254.169.254", true}, // cloud metadata / link-local
		{"169.254.0.1", true},
		{"100.64.0.1", true}, // CGNAT
		{"0.0.0.0", true},
		{"224.0.0.1", true}, // multicast
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"2001:4860:4860::8888", false},
	}

	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("failed to parse IP %q", tt.ip)
			}
			if got := IsBlockedIP(ip); got != tt.blocked {
				t.Errorf("IsBlockedIP(%s) = %v, want %v", tt.ip, got, tt.blocked)
			}
		})
	}
}

func TestValidateURL(t *testing.T) {
	tests := []struct {
		raw     string
		wantErr error
	}{
		{"https://example.com/path", nil},
		{"http://example.com", nil},
		{"ftp://example.com", ErrInvalidScheme},
		{"file:///etc/passwd", ErrInvalidScheme},
		{"http://127.0.0.1/", ErrSSRFBlocked},
		{"http://localhost/feed", ErrSSRFBlocked},
		{"http://169.254.169.254/latest/meta-data/", ErrSSRFBlocked},
		{"http://10.0.0.5/internal", ErrSSRFBlocked},
		{"http://[::1]/", ErrSSRFBlocked},
		{"http:///nohost", ErrSSRFBlocked},
	}

	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			u, err := url.Parse(tt.raw)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			err = ValidateURL(u)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestHasLiteralBlockedHost(t *testing.T) {
	if !HasLiteralBlockedHost("http://127.0.0.1/feed") {
		t.Error("expected 127.0.0.1 blocked")
	}
	if !HasLiteralBlockedHost("http://localhost/feed") {
		t.Error("expected localhost blocked")
	}
	if HasLiteralBlockedHost("https://feeds.example.com/rss") {
		t.Error("public hostname should not be blocked at config time")
	}
}

func TestSSRFBlocksLoopbackDial(t *testing.T) {
	c := New(WithRetryConfig(NoRetryConfig()))
	_, err := c.Get(context.Background(), "http://127.0.0.1:9/")
	if err == nil {
		t.Fatal("expected SSRF block for loopback")
	}
	if !errors.Is(err, ErrSSRFBlocked) && !strings.Contains(err.Error(), "ssrf") {
		t.Fatalf("expected SSRF error, got %v", err)
	}
}

func TestCheckRedirectRejectsPrivateHop(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:8080/secret", nil)
	if err != nil {
		t.Fatal(err)
	}
	viaReq, err := http.NewRequest(http.MethodGet, "https://example.com/", nil)
	if err != nil {
		t.Fatal(err)
	}
	err = checkRedirect(req, []*http.Request{viaReq})
	if !errors.Is(err, ErrSSRFBlocked) {
		t.Fatalf("error = %v, want ErrSSRFBlocked", err)
	}
}

func TestCheckRedirectRejectsNonHTTP(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "file:///etc/passwd", nil)
	if err != nil {
		t.Fatal(err)
	}
	viaReq, err := http.NewRequest(http.MethodGet, "https://example.com/", nil)
	if err != nil {
		t.Fatal(err)
	}
	err = checkRedirect(req, []*http.Request{viaReq})
	if !errors.Is(err, ErrInvalidScheme) {
		t.Fatalf("error = %v, want ErrInvalidScheme", err)
	}
}

func TestCheckRedirectTooMany(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://example.com/final", nil)
	if err != nil {
		t.Fatal(err)
	}
	via := make([]*http.Request, MaxRedirects)
	for i := range via {
		via[i], _ = http.NewRequest(http.MethodGet, "https://example.com/", nil)
	}
	err = checkRedirect(req, via)
	if !errors.Is(err, ErrTooManyRedirects) {
		t.Fatalf("error = %v, want ErrTooManyRedirects", err)
	}
}

func TestGetBodySizeCap(t *testing.T) {
	payload := strings.Repeat("x", 1024)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, payload)
	}))
	defer server.Close()

	c := New(
		WithAllowPrivateHosts(true),
		WithRetryConfig(NoRetryConfig()),
		WithMaxResponseBytes(100),
	)
	_, err := c.GetBody(context.Background(), server.URL)
	if !errors.Is(err, ErrResponseTooLarge) {
		t.Fatalf("error = %v, want ErrResponseTooLarge", err)
	}
}

func TestGetBodyUnderCap(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "ok")
	}))
	defer server.Close()

	c := New(
		WithAllowPrivateHosts(true),
		WithRetryConfig(NoRetryConfig()),
		WithMaxResponseBytes(100),
	)
	body, err := c.GetBody(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(body) != "ok" {
		t.Fatalf("body = %q", body)
	}
}

func TestAllowPrivateHostsBypassesSSRF(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "local")
	}))
	defer server.Close()

	c := New(WithAllowPrivateHosts(true), WithRetryConfig(NoRetryConfig()))
	body, err := c.GetBody(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(body) != "local" {
		t.Fatalf("body = %q", body)
	}
}
