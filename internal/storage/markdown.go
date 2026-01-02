package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/anthropic/autonomous-runner/internal/summarizer"
)

// MarkdownStorage stores articles as Markdown files.
type MarkdownStorage struct {
	config Config
}

// NewMarkdownStorage creates a new Markdown storage instance.
func NewMarkdownStorage(config Config) *MarkdownStorage {
	return &MarkdownStorage{config: config}
}

// NewMarkdownStorageDefault creates storage with default config.
func NewMarkdownStorageDefault() *MarkdownStorage {
	return NewMarkdownStorage(DefaultConfig())
}

// Save persists articles to a daily archive file.
func (s *MarkdownStorage) Save(articles []summarizer.EnrichedArticle) error {
	archiveDir := filepath.Join(s.config.BaseDir, s.config.ArchiveDir)
	if err := os.MkdirAll(archiveDir, 0755); err != nil {
		return fmt.Errorf("create archive dir: %w", err)
	}

	today := time.Now().Format("2006-01-02")
	archiveFile := filepath.Join(archiveDir, today+".md")

	content := s.articlesToMarkdown(articles, today)
	if err := os.WriteFile(archiveFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("write archive: %w", err)
	}

	return nil
}

// SaveReport writes the daily report to the digest file.
func (s *MarkdownStorage) SaveReport(report summarizer.Report) error {
	if err := os.MkdirAll(s.config.BaseDir, 0755); err != nil {
		return fmt.Errorf("create base dir: %w", err)
	}

	digestPath := filepath.Join(s.config.BaseDir, s.config.DigestFile)
	if err := os.WriteFile(digestPath, []byte(report.Content), 0644); err != nil {
		return fmt.Errorf("write digest: %w", err)
	}

	return nil
}

// Load retrieves articles from archive files.
func (s *MarkdownStorage) Load(since time.Time) ([]summarizer.EnrichedArticle, error) {
	// Not implemented for MVP - returns empty list
	return nil, nil
}

func (s *MarkdownStorage) articlesToMarkdown(articles []summarizer.EnrichedArticle, date string) string {
	md := fmt.Sprintf("# Tech Digest Archive - %s\n\n", date)
	md += fmt.Sprintf("Total articles: %d\n\n", len(articles))

	// Group by source
	bySource := make(map[string][]summarizer.EnrichedArticle)
	for _, a := range articles {
		bySource[a.Source] = append(bySource[a.Source], a)
	}

	for source, sourceArticles := range bySource {
		md += fmt.Sprintf("## %s (%d articles)\n\n", source, len(sourceArticles))

		for _, a := range sourceArticles {
			md += fmt.Sprintf("### [%s](%s)\n", a.Title, a.URL)
			md += fmt.Sprintf("- Score: %d | Comments: %d | Importance: %.1f/10\n",
				a.Score, a.Comments, a.Importance)
			if len(a.MatchedKeywords) > 0 {
				md += fmt.Sprintf("- Keywords: %v\n", a.MatchedKeywords)
			}
			if a.Author != "" {
				md += fmt.Sprintf("- Author: %s\n", a.Author)
			}
			md += "\n"
		}
	}

	md += fmt.Sprintf("\n---\nGenerated at: %s\n", time.Now().Format(time.RFC3339))
	return md
}
