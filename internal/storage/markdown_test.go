package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/anthropic/autonomous-runner/internal/collector"
	"github.com/anthropic/autonomous-runner/internal/filter"
	"github.com/anthropic/autonomous-runner/internal/summarizer"
)

func TestNewMarkdownStorage(t *testing.T) {
	config := Config{
		BaseDir:    "test-dir",
		DigestFile: "test.md",
	}

	storage := NewMarkdownStorage(config)

	if storage == nil {
		t.Fatal("expected non-nil storage")
	}

	if storage.config.BaseDir != "test-dir" {
		t.Errorf("expected BaseDir 'test-dir', got '%s'", storage.config.BaseDir)
	}
}

func TestNewMarkdownStorageDefault(t *testing.T) {
	storage := NewMarkdownStorageDefault()

	if storage == nil {
		t.Fatal("expected non-nil storage")
	}

	if storage.config.BaseDir != ".techpulse" {
		t.Errorf("expected BaseDir '.techpulse', got '%s'", storage.config.BaseDir)
	}
}

func TestMarkdownStorageSave(t *testing.T) {
	tmpDir := t.TempDir()
	config := Config{
		BaseDir:    tmpDir,
		ArchiveDir: "archive",
	}
	storage := NewMarkdownStorage(config)

	articles := []summarizer.EnrichedArticle{
		createTestEnrichedArticle("1", "Test Article 1", "hackernews", 8.5),
		createTestEnrichedArticle("2", "Test Article 2", "rss", 7.0),
	}

	err := storage.Save(articles)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify archive directory was created
	archiveDir := filepath.Join(tmpDir, "archive")
	if _, err := os.Stat(archiveDir); os.IsNotExist(err) {
		t.Error("archive directory was not created")
	}

	// Verify archive file was created
	today := time.Now().Format("2006-01-02")
	archiveFile := filepath.Join(archiveDir, today+".md")
	if _, err := os.Stat(archiveFile); os.IsNotExist(err) {
		t.Errorf("archive file was not created: %s", archiveFile)
	}

	// Verify content
	content, err := os.ReadFile(archiveFile)
	if err != nil {
		t.Fatalf("failed to read archive file: %v", err)
	}

	if !strings.Contains(string(content), "Test Article 1") {
		t.Error("archive file should contain 'Test Article 1'")
	}

	if !strings.Contains(string(content), "Test Article 2") {
		t.Error("archive file should contain 'Test Article 2'")
	}
}

func TestMarkdownStorageSaveReport(t *testing.T) {
	tmpDir := t.TempDir()
	config := Config{
		BaseDir:    tmpDir,
		DigestFile: "DIGEST.md",
	}
	storage := NewMarkdownStorage(config)

	report := summarizer.Report{
		Title:   "Test Report",
		Date:    "2026-01-01",
		Content: "# Test Report\n\nThis is a test report.",
	}

	err := storage.SaveReport(report)
	if err != nil {
		t.Fatalf("SaveReport failed: %v", err)
	}

	// Verify digest file was created
	digestFile := filepath.Join(tmpDir, "DIGEST.md")
	if _, err := os.Stat(digestFile); os.IsNotExist(err) {
		t.Error("digest file was not created")
	}

	// Verify content
	content, err := os.ReadFile(digestFile)
	if err != nil {
		t.Fatalf("failed to read digest file: %v", err)
	}

	if string(content) != report.Content {
		t.Errorf("expected content '%s', got '%s'", report.Content, string(content))
	}
}

func TestMarkdownStorageLoad(t *testing.T) {
	storage := NewMarkdownStorageDefault()

	// Load should return nil for MVP (not implemented)
	articles, err := storage.Load(time.Now().Add(-24 * time.Hour))

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if articles != nil {
		t.Errorf("expected nil articles for MVP, got %v", articles)
	}
}

func TestMarkdownStorageSaveEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	config := Config{
		BaseDir:    tmpDir,
		ArchiveDir: "archive",
	}
	storage := NewMarkdownStorage(config)

	err := storage.Save([]summarizer.EnrichedArticle{})
	if err != nil {
		t.Fatalf("Save with empty articles failed: %v", err)
	}

	// Verify file was created with proper header
	today := time.Now().Format("2006-01-02")
	archiveFile := filepath.Join(tmpDir, "archive", today+".md")
	content, _ := os.ReadFile(archiveFile)

	if !strings.Contains(string(content), "Total articles: 0") {
		t.Error("archive file should indicate 0 articles")
	}
}

func TestArticlesToMarkdownGroupsBySource(t *testing.T) {
	tmpDir := t.TempDir()
	config := Config{
		BaseDir:    tmpDir,
		ArchiveDir: "archive",
	}
	storage := NewMarkdownStorage(config)

	articles := []summarizer.EnrichedArticle{
		createTestEnrichedArticle("1", "HN Article 1", "hackernews", 8.0),
		createTestEnrichedArticle("2", "RSS Article 1", "rss", 7.5),
		createTestEnrichedArticle("3", "HN Article 2", "hackernews", 6.0),
	}

	err := storage.Save(articles)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	today := time.Now().Format("2006-01-02")
	archiveFile := filepath.Join(tmpDir, "archive", today+".md")
	content, _ := os.ReadFile(archiveFile)
	contentStr := string(content)

	// Verify grouping headers
	if !strings.Contains(contentStr, "## hackernews") {
		t.Error("should contain hackernews source header")
	}

	if !strings.Contains(contentStr, "## rss") {
		t.Error("should contain rss source header")
	}
}

func TestArticlesToMarkdownIncludesMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	config := Config{
		BaseDir:    tmpDir,
		ArchiveDir: "archive",
	}
	storage := NewMarkdownStorage(config)

	article := createTestEnrichedArticle("1", "Test Article", "hackernews", 8.5)
	article.Score = 150
	article.Comments = 42
	article.MatchedKeywords = []string{"AI", "LLM"}
	article.Author = "testuser"

	err := storage.Save([]summarizer.EnrichedArticle{article})
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	today := time.Now().Format("2006-01-02")
	archiveFile := filepath.Join(tmpDir, "archive", today+".md")
	content, _ := os.ReadFile(archiveFile)
	contentStr := string(content)

	if !strings.Contains(contentStr, "Score: 150") {
		t.Error("should contain score")
	}

	if !strings.Contains(contentStr, "Comments: 42") {
		t.Error("should contain comments count")
	}

	if !strings.Contains(contentStr, "Importance: 8.5/10") {
		t.Error("should contain importance score")
	}

	if !strings.Contains(contentStr, "[AI LLM]") {
		t.Error("should contain matched keywords")
	}

	if !strings.Contains(contentStr, "Author: testuser") {
		t.Error("should contain author")
	}
}

// Helper function to create test enriched articles
func createTestEnrichedArticle(id, title, source string, importance float64) summarizer.EnrichedArticle {
	return summarizer.EnrichedArticle{
		FilteredArticle: filter.FilteredArticle{
			Article: collector.Article{
				ID:          id,
				Title:       title,
				URL:         "https://example.com/" + id,
				Source:      source,
				PublishedAt: time.Now(),
				CollectedAt: time.Now(),
			},
		},
		Importance: importance,
	}
}
