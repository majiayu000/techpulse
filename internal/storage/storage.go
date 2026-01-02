// Package storage provides persistence for collected articles.
package storage

import (
	"time"

	"github.com/anthropic/autonomous-runner/internal/summarizer"
)

// Storage defines the interface for article persistence.
type Storage interface {
	// Save persists enriched articles.
	Save(articles []summarizer.EnrichedArticle) error

	// SaveReport persists the daily report.
	SaveReport(report summarizer.Report) error

	// Load retrieves articles since the given time.
	Load(since time.Time) ([]summarizer.EnrichedArticle, error)
}

// Config contains storage configuration.
type Config struct {
	BaseDir       string
	DigestFile    string
	ArchiveDir    string
	RetentionDays int
}

// DefaultConfig returns the default storage configuration.
func DefaultConfig() Config {
	return Config{
		BaseDir:       ".techpulse",
		DigestFile:    "DIGEST.md",
		ArchiveDir:    "archive",
		RetentionDays: 30,
	}
}
