// Configuration validation for TechPulse.
package techpulse

import (
	"fmt"
	"net/url"
	"strings"
)

// ValidationError represents a configuration validation error.
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidationErrors is a collection of validation errors.
type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return ""
	}
	var msgs []string
	for _, err := range e {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// ValidSources contains all valid data source names.
var ValidSources = map[string]bool{
	"hackernews_top":        true,
	"hackernews_ask":        true,
	"hackernews_show":       true,
	"rss":                   true,
	"github_trending_daily": true,
	"reddit":                true,
	"lobsters_hottest":      true,
}

// ValidateConfig validates the runtime configuration.
func ValidateConfig(cfg Config) ValidationErrors {
	var errs ValidationErrors

	if cfg.Limit < 1 || cfg.Limit > 500 {
		errs = append(errs, ValidationError{
			Field:   "limit",
			Message: "must be between 1 and 500",
		})
	}

	if cfg.Timeout < 1 || cfg.Timeout > 300 {
		errs = append(errs, ValidationError{
			Field:   "timeout",
			Message: "must be between 1 and 300 seconds",
		})
	}

	if cfg.RetentionDays < 0 || cfg.RetentionDays > 3650 {
		errs = append(errs, ValidationError{
			Field:   "retention_days",
			Message: "must be between 0 and 3650 (0 disables cleanup)",
		})
	}

	if cfg.Output == "" {
		errs = append(errs, ValidationError{
			Field:   "output",
			Message: "cannot be empty",
		})
	}

	// Validate sources
	for _, src := range cfg.Sources {
		if !ValidSources[src] {
			errs = append(errs, ValidationError{
				Field:   "sources",
				Message: fmt.Sprintf("unknown source: %s", src),
			})
		}
	}

	return errs
}

// ValidateFileConfig validates the YAML configuration file.
func ValidateFileConfig(cfg *FileConfig) ValidationErrors {
	if cfg == nil {
		return nil
	}

	var errs ValidationErrors

	// Validate limit
	if cfg.Limit < 0 || cfg.Limit > 500 {
		errs = append(errs, ValidationError{
			Field:   "limit",
			Message: "must be between 0 and 500 (0 uses default)",
		})
	}

	// Validate timeout
	if cfg.Timeout < 0 || cfg.Timeout > 300 {
		errs = append(errs, ValidationError{
			Field:   "timeout",
			Message: "must be between 0 and 300 seconds (0 uses default)",
		})
	}

	if cfg.RetentionDays != nil && (*cfg.RetentionDays < 0 || *cfg.RetentionDays > 3650) {
		errs = append(errs, ValidationError{
			Field:   "retention_days",
			Message: "must be between 0 and 3650 (0 disables cleanup)",
		})
	}

	// Validate sources
	for _, src := range cfg.Sources {
		if !ValidSources[src] {
			errs = append(errs, ValidationError{
				Field:   "sources",
				Message: fmt.Sprintf("unknown source: %s (valid: %s)", src, validSourceList()),
			})
		}
	}

	// Validate RSS feeds
	for i, feed := range cfg.RSSFeeds {
		if feed.Name == "" {
			errs = append(errs, ValidationError{
				Field:   fmt.Sprintf("rss_feeds[%d].name", i),
				Message: "cannot be empty",
			})
		}
		if feed.URL == "" {
			errs = append(errs, ValidationError{
				Field:   fmt.Sprintf("rss_feeds[%d].url", i),
				Message: "cannot be empty",
			})
		} else if !isValidURL(feed.URL) {
			errs = append(errs, ValidationError{
				Field:   fmt.Sprintf("rss_feeds[%d].url", i),
				Message: "must be a valid HTTP/HTTPS URL",
			})
		}
	}

	return errs
}

// isValidURL checks if a string is a valid HTTP/HTTPS URL.
func isValidURL(s string) bool {
	u, err := url.Parse(s)
	if err != nil {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}

// validSourceList returns a comma-separated list of valid sources.
func validSourceList() string {
	sources := make([]string, 0, len(ValidSources))
	for src := range ValidSources {
		sources = append(sources, src)
	}
	return strings.Join(sources, ", ")
}
