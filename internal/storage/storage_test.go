package storage

import (
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.BaseDir != ".techpulse" {
		t.Errorf("expected BaseDir '.techpulse', got '%s'", config.BaseDir)
	}

	if config.DigestFile != "DIGEST.md" {
		t.Errorf("expected DigestFile 'DIGEST.md', got '%s'", config.DigestFile)
	}

	if config.ArchiveDir != "archive" {
		t.Errorf("expected ArchiveDir 'archive', got '%s'", config.ArchiveDir)
	}

	if config.RetentionDays != 30 {
		t.Errorf("expected RetentionDays 30, got %d", config.RetentionDays)
	}
}

func TestConfigCustomValues(t *testing.T) {
	config := Config{
		BaseDir:       "custom",
		DigestFile:    "daily.md",
		ArchiveDir:    "history",
		RetentionDays: 7,
	}

	if config.BaseDir != "custom" {
		t.Errorf("expected BaseDir 'custom', got '%s'", config.BaseDir)
	}

	if config.DigestFile != "daily.md" {
		t.Errorf("expected DigestFile 'daily.md', got '%s'", config.DigestFile)
	}

	if config.ArchiveDir != "history" {
		t.Errorf("expected ArchiveDir 'history', got '%s'", config.ArchiveDir)
	}

	if config.RetentionDays != 7 {
		t.Errorf("expected RetentionDays 7, got %d", config.RetentionDays)
	}
}
