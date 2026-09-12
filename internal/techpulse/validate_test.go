package techpulse

import (
	"strings"
	"testing"
)

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name      string
		cfg       Config
		wantErrs  int
		wantField string
	}{
		{
			name:     "valid default config",
			cfg:      DefaultConfig(),
			wantErrs: 0,
		},
		{
			name:      "limit too low",
			cfg:       Config{Limit: 0, Timeout: 60, Output: ".techpulse"},
			wantErrs:  1,
			wantField: "limit",
		},
		{
			name:      "limit too high",
			cfg:       Config{Limit: 501, Timeout: 60, Output: ".techpulse"},
			wantErrs:  1,
			wantField: "limit",
		},
		{
			name:      "timeout too low",
			cfg:       Config{Limit: 30, Timeout: 0, Output: ".techpulse"},
			wantErrs:  1,
			wantField: "timeout",
		},
		{
			name:      "timeout too high",
			cfg:       Config{Limit: 30, Timeout: 301, Output: ".techpulse"},
			wantErrs:  1,
			wantField: "timeout",
		},
		{
			name:      "empty output",
			cfg:       Config{Limit: 30, Timeout: 60, Output: ""},
			wantErrs:  1,
			wantField: "output",
		},
		{
			name:      "invalid source",
			cfg:       Config{Limit: 30, Timeout: 60, Output: ".techpulse", Sources: []string{"invalid_source"}},
			wantErrs:  1,
			wantField: "sources",
		},
		{
			name:     "valid sources",
			cfg:      Config{Limit: 30, Timeout: 60, Output: ".techpulse", Sources: []string{"hackernews_top", "rss"}},
			wantErrs: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidateConfig(tt.cfg)
			if len(errs) != tt.wantErrs {
				t.Errorf("ValidateConfig() got %d errors, want %d: %v", len(errs), tt.wantErrs, errs)
			}
			if tt.wantField != "" && len(errs) > 0 {
				if errs[0].Field != tt.wantField {
					t.Errorf("ValidateConfig() error field = %s, want %s", errs[0].Field, tt.wantField)
				}
			}
		})
	}
}

func TestValidateFileConfig(t *testing.T) {
	tests := []struct {
		name      string
		cfg       *FileConfig
		wantErrs  int
		wantField string
	}{
		{
			name:     "nil config",
			cfg:      nil,
			wantErrs: 0,
		},
		{
			name:     "empty config",
			cfg:      &FileConfig{},
			wantErrs: 0,
		},
		{
			name:      "limit too high",
			cfg:       &FileConfig{Limit: 501},
			wantErrs:  1,
			wantField: "limit",
		},
		{
			name:      "negative limit",
			cfg:       &FileConfig{Limit: -1},
			wantErrs:  1,
			wantField: "limit",
		},
		{
			name:      "timeout too high",
			cfg:       &FileConfig{Timeout: 301},
			wantErrs:  1,
			wantField: "timeout",
		},
		{
			name:      "invalid source",
			cfg:       &FileConfig{Sources: []string{"unknown"}},
			wantErrs:  1,
			wantField: "sources",
		},
		{
			name: "valid RSS feed",
			cfg: &FileConfig{
				RSSFeeds: []RSSFeedConfig{{Name: "Test", URL: "https://example.com/feed"}},
			},
			wantErrs: 0,
		},
		{
			name: "private RSS feed IP",
			cfg: &FileConfig{
				RSSFeeds: []RSSFeedConfig{{Name: "Internal", URL: "http://127.0.0.1/feed"}},
			},
			wantErrs:  1,
			wantField: "rss_feeds[0].url",
		},
		{
			name: "localhost RSS feed",
			cfg: &FileConfig{
				RSSFeeds: []RSSFeedConfig{{Name: "Local", URL: "http://localhost/feed"}},
			},
			wantErrs:  1,
			wantField: "rss_feeds[0].url",
		},
		{
			name: "metadata IP RSS feed",
			cfg: &FileConfig{
				RSSFeeds: []RSSFeedConfig{{Name: "Meta", URL: "http://169.254.169.254/latest"}},
			},
			wantErrs:  1,
			wantField: "rss_feeds[0].url",
		},
		{
			name: "empty RSS feed name",
			cfg: &FileConfig{
				RSSFeeds: []RSSFeedConfig{{Name: "", URL: "https://example.com/feed"}},
			},
			wantErrs:  1,
			wantField: "rss_feeds[0].name",
		},
		{
			name: "empty RSS feed URL",
			cfg: &FileConfig{
				RSSFeeds: []RSSFeedConfig{{Name: "Test", URL: ""}},
			},
			wantErrs:  1,
			wantField: "rss_feeds[0].url",
		},
		{
			name: "invalid RSS feed URL",
			cfg: &FileConfig{
				RSSFeeds: []RSSFeedConfig{{Name: "Test", URL: "not-a-url"}},
			},
			wantErrs:  1,
			wantField: "rss_feeds[0].url",
		},
		{
			name: "multiple errors",
			cfg: &FileConfig{
				Limit:   -1,
				Timeout: 999,
				Sources: []string{"bad_source"},
			},
			wantErrs: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidateFileConfig(tt.cfg)
			if len(errs) != tt.wantErrs {
				t.Errorf("ValidateFileConfig() got %d errors, want %d: %v", len(errs), tt.wantErrs, errs)
			}
			if tt.wantField != "" && len(errs) > 0 {
				if errs[0].Field != tt.wantField {
					t.Errorf("ValidateFileConfig() error field = %s, want %s", errs[0].Field, tt.wantField)
				}
			}
		})
	}
}

func TestValidationError(t *testing.T) {
	err := ValidationError{Field: "limit", Message: "must be positive"}
	if !strings.Contains(err.Error(), "limit") || !strings.Contains(err.Error(), "positive") {
		t.Errorf("ValidationError.Error() = %q, want to contain field and message", err.Error())
	}
}

func TestValidationErrors(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		var errs ValidationErrors
		if errs.Error() != "" {
			t.Errorf("Empty ValidationErrors.Error() = %q, want empty", errs.Error())
		}
	})

	t.Run("multiple", func(t *testing.T) {
		errs := ValidationErrors{
			{Field: "a", Message: "error1"},
			{Field: "b", Message: "error2"},
		}
		msg := errs.Error()
		if !strings.Contains(msg, "a") || !strings.Contains(msg, "b") {
			t.Errorf("ValidationErrors.Error() = %q, want to contain both errors", msg)
		}
	})
}

func TestIsValidURL(t *testing.T) {
	tests := []struct {
		url  string
		want bool
	}{
		{"https://example.com", true},
		{"http://example.com/feed.xml", true},
		{"ftp://example.com", false},
		{"not-a-url", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			if got := isValidURL(tt.url); got != tt.want {
				t.Errorf("isValidURL(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}

func TestValidSourceList(t *testing.T) {
	list := validSourceList()
	// Should contain all valid sources
	for src := range ValidSources {
		if !strings.Contains(list, src) {
			t.Errorf("validSourceList() = %q, want to contain %s", list, src)
		}
	}
}
