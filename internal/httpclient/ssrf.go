package httpclient

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultMaxResponseBytes caps response bodies read by GetBody (5 MiB).
const DefaultMaxResponseBytes int64 = 5 << 20

// MaxRedirects is the maximum number of redirects the SSRF-safe client will follow.
const MaxRedirects = 10

var (
	// ErrSSRFBlocked is returned when a request targets a disallowed address.
	ErrSSRFBlocked = errors.New("ssrf: blocked destination")
	// ErrInvalidScheme is returned when a URL scheme is not http or https.
	ErrInvalidScheme = errors.New("ssrf: only http and https URLs are allowed")
	// ErrResponseTooLarge is returned when a response body exceeds the size cap.
	ErrResponseTooLarge = errors.New("httpclient: response body exceeds size limit")
	// ErrTooManyRedirects is returned when redirect hops exceed MaxRedirects.
	ErrTooManyRedirects = errors.New("ssrf: too many redirects")
)

// cgnatCIDR is RFC 6598 shared address space (100.64.0.0/10).
var cgnatCIDR = mustCIDR("100.64.0.0/10")

func mustCIDR(s string) *net.IPNet {
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		panic(err)
	}
	return n
}

// IsBlockedIP reports whether ip is unsuitable for outbound fetches
// (loopback, private, link-local, multicast, unspecified, or CGNAT).
func IsBlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}
	if cgnatCIDR.Contains(ip) {
		return true
	}
	return false
}

// ValidateURL checks scheme and literal host policy before dialing.
// Hostnames are re-checked after DNS resolution in DialContext.
func ValidateURL(u *url.URL) error {
	if u == nil {
		return fmt.Errorf("%w: missing URL", ErrSSRFBlocked)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("%w: %q", ErrInvalidScheme, u.Scheme)
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("%w: missing host", ErrSSRFBlocked)
	}
	if strings.EqualFold(host, "localhost") {
		return fmt.Errorf("%w: localhost", ErrSSRFBlocked)
	}
	if ip := net.ParseIP(host); ip != nil {
		if IsBlockedIP(ip) {
			return fmt.Errorf("%w: %s", ErrSSRFBlocked, ip)
		}
	}
	return nil
}

// HasLiteralBlockedHost reports whether rawURL uses localhost or a literal
// blocked IP. Used for config-time defense-in-depth (no DNS lookup).
func HasLiteralBlockedHost(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return true
	}
	host := u.Hostname()
	if host == "" {
		return true
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return IsBlockedIP(ip)
	}
	return false
}

func safeDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}

	dialer := &net.Dialer{Timeout: 30 * time.Second}

	if ip := net.ParseIP(host); ip != nil {
		if IsBlockedIP(ip) {
			return nil, fmt.Errorf("%w: %s", ErrSSRFBlocked, ip)
		}
		return dialer.DialContext(ctx, network, addr)
	}

	ipAddrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}

	var lastErr error
	for _, ipAddr := range ipAddrs {
		if IsBlockedIP(ipAddr.IP) {
			lastErr = fmt.Errorf("%w: %s resolved to %s", ErrSSRFBlocked, host, ipAddr.IP)
			continue
		}
		target := net.JoinHostPort(ipAddr.IP.String(), port)
		conn, err := dialer.DialContext(ctx, network, target)
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("%w: no usable addresses for %s", ErrSSRFBlocked, host)
}

func checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= MaxRedirects {
		return ErrTooManyRedirects
	}
	if err := ValidateURL(req.URL); err != nil {
		return err
	}
	return nil
}

func ssrfTransport() *http.Transport {
	// Clone DefaultTransport settings so we keep proxy/TLS defaults,
	// then replace DialContext with the SSRF-safe dialer.
	base, ok := http.DefaultTransport.(*http.Transport)
	var tr *http.Transport
	if ok {
		tr = base.Clone()
	} else {
		tr = &http.Transport{}
	}
	tr.DialContext = safeDialContext
	return tr
}
