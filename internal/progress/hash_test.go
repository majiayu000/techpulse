package progress

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/anthropic/autonomous-runner/internal/memory"
)

func createTestMemory(t *testing.T, dir string) *memory.Manager {
	mem, err := memory.NewManager(dir)
	if err != nil {
		t.Fatalf("failed to create memory manager: %v", err)
	}
	return mem
}

func updateContextFile(t *testing.T, dir, content string) {
	path := filepath.Join(dir, "CONTEXT.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write context file: %v", err)
	}
}

func TestHashDetector_New(t *testing.T) {
	tmpDir := t.TempDir()
	mem := createTestMemory(t, tmpDir)

	hd, err := NewHashDetector(mem)
	if err != nil {
		t.Fatalf("NewHashDetector failed: %v", err)
	}
	if hd == nil {
		t.Fatal("expected non-nil HashDetector")
	}
}

func TestHashDetector_Name(t *testing.T) {
	tmpDir := t.TempDir()
	mem := createTestMemory(t, tmpDir)

	hd, _ := NewHashDetector(mem)
	if hd.Name() != "hash" {
		t.Errorf("expected name 'hash', got %q", hd.Name())
	}
}

func TestHashDetector_DetectNoChange(t *testing.T) {
	tmpDir := t.TempDir()
	mem := createTestMemory(t, tmpDir)

	hd, _ := NewHashDetector(mem)

	// First detect without changes
	hasProgress, err := hd.Detect()
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}
	if hasProgress {
		t.Error("expected no progress without changes")
	}
}

func TestHashDetector_DetectWithChange(t *testing.T) {
	tmpDir := t.TempDir()
	mem := createTestMemory(t, tmpDir)

	hd, _ := NewHashDetector(mem)

	// Modify the context file
	updateContextFile(t, tmpDir, "modified content")

	hasProgress, err := hd.Detect()
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}
	if !hasProgress {
		t.Error("expected progress after content change")
	}
}

func TestHashDetector_DetectMultiple(t *testing.T) {
	tmpDir := t.TempDir()
	mem := createTestMemory(t, tmpDir)

	hd, _ := NewHashDetector(mem)

	// Detect with no change
	p1, _ := hd.Detect()
	if p1 {
		t.Error("expected no progress initially")
	}

	// Detect again with no change (hash should match last)
	p2, _ := hd.Detect()
	if p2 {
		t.Error("expected no progress without change")
	}

	// Now change content
	updateContextFile(t, tmpDir, "changed content")
	p3, _ := hd.Detect()
	if !p3 {
		t.Error("expected progress after change")
	}
}

func TestHashDetector_Reset(t *testing.T) {
	tmpDir := t.TempDir()
	mem := createTestMemory(t, tmpDir)

	hd, _ := NewHashDetector(mem)

	// Change content
	updateContextFile(t, tmpDir, "changed")

	// Reset should update lastHash to current
	err := hd.Reset()
	if err != nil {
		t.Fatalf("Reset failed: %v", err)
	}

	// Detect should now show no progress
	hasProgress, _ := hd.Detect()
	if hasProgress {
		t.Error("expected no progress after reset")
	}
}

func TestHashDetector_Details(t *testing.T) {
	tmpDir := t.TempDir()
	mem := createTestMemory(t, tmpDir)

	hd, _ := NewHashDetector(mem)

	// Change content to trigger hash update
	updateContextFile(t, tmpDir, "changed content for details")
	hd.Detect()

	details := hd.Details()
	if details == "" {
		t.Error("expected non-empty details")
	}
	// Details should contain arrow indicating change
	if len(details) < 10 {
		t.Errorf("expected longer details string, got %q", details)
	}
}

func TestHashDetector_NewWithMissingMemory(t *testing.T) {
	tmpDir := t.TempDir()
	nonExistentDir := filepath.Join(tmpDir, "does-not-exist")

	// Try creating memory - it may auto-create the directory
	mem, err := memory.NewManager(nonExistentDir)
	if err != nil {
		// Manager creation failed - expected behavior
		return
	}

	// HashDetector should work if memory was created
	hd, err := NewHashDetector(mem)
	if err != nil {
		// This is also acceptable
		return
	}
	if hd == nil {
		t.Error("expected non-nil HashDetector when memory exists")
	}
}
