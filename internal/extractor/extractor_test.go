package extractor

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/anthropic/autonomous-runner/internal/httpclient"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.MaxLength != 200 {
		t.Errorf("MaxLength = %d, want 200", cfg.MaxLength)
	}
	if !cfg.FetchRemote {
		t.Error("FetchRemote should be true by default")
	}
}

func TestExtract_ExistingContent(t *testing.T) {
	client := httpclient.New()
	ext := NewDefault(client)

	summary, err := ext.Extract(context.Background(), "http://example.com", "This is existing content")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if summary != "This is existing content" {
		t.Errorf("summary = %q, want %q", summary, "This is existing content")
	}
}

func TestExtract_TruncateLongContent(t *testing.T) {
	client := httpclient.New()
	cfg := Config{MaxLength: 20, FetchRemote: false}
	ext := New(client, cfg)

	content := "This is a very long content that should be truncated"
	summary, err := ext.Extract(context.Background(), "http://example.com", content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "This is a very long ..."
	if summary != expected {
		t.Errorf("summary = %q, want %q", summary, expected)
	}
}

func TestExtract_FetchRemote(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`
			<html>
			<head><title>Test</title></head>
			<body>
				<p>This is the article content that we want to extract.</p>
			</body>
			</html>
		`))
	}))
	defer server.Close()

	client := httpclient.New()
	ext := NewDefault(client)

	summary, err := ext.Extract(context.Background(), server.URL, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if summary == "" {
		t.Error("summary should not be empty")
	}

	if len(summary) > 203 { // 200 + "..."
		t.Errorf("summary too long: %d chars", len(summary))
	}
}

func TestExtract_FetchDisabled(t *testing.T) {
	client := httpclient.New()
	cfg := Config{MaxLength: 200, FetchRemote: false}
	ext := New(client, cfg)

	summary, err := ext.Extract(context.Background(), "http://example.com", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if summary != "" {
		t.Errorf("summary = %q, want empty when fetch disabled", summary)
	}
}

func TestExtract_FailedFetchReturnsEmpty(t *testing.T) {
	client := httpclient.New()
	ext := NewDefault(client)

	// Use invalid URL that will fail
	summary, err := ext.Extract(context.Background(), "http://invalid.invalid.invalid", "")
	if err != nil {
		t.Fatalf("should not return error on failed fetch: %v", err)
	}

	if summary != "" {
		t.Errorf("summary = %q, want empty on failed fetch", summary)
	}
}
