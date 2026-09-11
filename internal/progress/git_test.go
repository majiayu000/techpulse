package progress

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGitDetector_NewInGitRepo(t *testing.T) {
	// Use current repo as test since it's a git repo
	wd, err := os.Getwd()
	if err != nil {
		t.Skipf("cannot get working directory: %v", err)
	}

	gd, err := NewGitDetector(wd)
	if err != nil {
		t.Skipf("test directory not a git repo: %v", err)
	}
	if gd == nil {
		t.Fatal("expected non-nil GitDetector")
	}
}

func TestGitDetector_Name(t *testing.T) {
	wd, _ := os.Getwd()
	gd, err := NewGitDetector(wd)
	if err != nil {
		t.Skipf("test skipped: %v", err)
	}

	if gd.Name() != "git" {
		t.Errorf("expected name 'git', got %q", gd.Name())
	}
}

func TestGitDetector_DetectNoChanges(t *testing.T) {
	wd, _ := os.Getwd()
	gd, err := NewGitDetector(wd)
	if err != nil {
		t.Skipf("test skipped: %v", err)
	}

	// Reset first to get clean state
	gd.Reset()

	// Detect in clean state (may or may not show changes based on current repo state)
	_, detectErr := gd.Detect()
	if detectErr != nil {
		t.Fatalf("Detect failed: %v", detectErr)
	}
}

func TestGitDetector_DetectWithNewFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Initialize a git repo in temp dir
	gd, err := NewGitDetector(tmpDir)
	if err != nil {
		t.Fatalf("NewGitDetector failed: %v", err)
	}

	// Reset to clean state
	gd.Reset()

	// Create a new file
	newFile := filepath.Join(tmpDir, "testfile.txt")
	if err := os.WriteFile(newFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	// Detect should find untracked file
	hasProgress, err := gd.Detect()
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}
	if !hasProgress {
		t.Error("expected progress with new untracked file")
	}
}

func TestGitDetector_PersistentDirtyIsNotProgress(t *testing.T) {
	tmpDir := t.TempDir()
	gd, err := NewGitDetector(tmpDir)
	if err != nil {
		t.Fatalf("NewGitDetector failed: %v", err)
	}

	dirty := filepath.Join(tmpDir, "leftover.txt")
	if err := os.WriteFile(dirty, []byte("stale"), 0644); err != nil {
		t.Fatalf("failed to create dirty file: %v", err)
	}

	// Baseline includes the dirty file; unchanged dirty state must not look like progress.
	if err := gd.Reset(); err != nil {
		t.Fatalf("Reset failed: %v", err)
	}
	hasProgress, err := gd.Detect()
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}
	if hasProgress {
		t.Fatal("expected no progress when dirty files are unchanged since Reset")
	}

	// A new path in the status snapshot after Reset should count as progress.
	if err := gd.Reset(); err != nil {
		t.Fatalf("second Reset failed: %v", err)
	}
	extra := filepath.Join(tmpDir, "extra.txt")
	if err := os.WriteFile(extra, []byte("new"), 0644); err != nil {
		t.Fatalf("failed to create extra file: %v", err)
	}
	hasProgress, err = gd.Detect()
	if err != nil {
		t.Fatalf("Detect after change failed: %v", err)
	}
	if !hasProgress {
		t.Fatal("expected progress when git status gains a new path after Reset")
	}
}

func TestGitDetector_DirtyPathContentEditIsProgress(t *testing.T) {
	tmpDir := t.TempDir()
	gd, err := NewGitDetector(tmpDir)
	if err != nil {
		t.Fatalf("NewGitDetector failed: %v", err)
	}

	dirty := filepath.Join(tmpDir, "tracked-dirty.txt")
	if err := os.WriteFile(dirty, []byte("v1"), 0644); err != nil {
		t.Fatalf("create dirty file: %v", err)
	}
	if err := gd.Reset(); err != nil {
		t.Fatalf("Reset failed: %v", err)
	}

	// Same porcelain entry (?? or M) but different contents must count as progress.
	if err := os.WriteFile(dirty, []byte("v2-edited-during-worker"), 0644); err != nil {
		t.Fatalf("edit dirty file: %v", err)
	}
	hasProgress, err := gd.Detect()
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}
	if !hasProgress {
		t.Fatal("expected progress when an already-dirty path's contents change")
	}
}

func TestGitDetector_UntrackedDirNestedEditIsProgress(t *testing.T) {
	tmpDir := t.TempDir()
	gd, err := NewGitDetector(tmpDir)
	if err != nil {
		t.Fatalf("NewGitDetector failed: %v", err)
	}

	dir := filepath.Join(tmpDir, "newdir")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	nested := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(nested, []byte("v1"), 0644); err != nil {
		t.Fatalf("write nested: %v", err)
	}
	if err := gd.Reset(); err != nil {
		t.Fatalf("Reset: %v", err)
	}

	// Edit under an already-untracked directory; default porcelain only shows ?? newdir/.
	if err := os.WriteFile(nested, []byte("v2-nested-edit"), 0644); err != nil {
		t.Fatalf("edit nested: %v", err)
	}
	hasProgress, err := gd.Detect()
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if !hasProgress {
		t.Fatal("expected progress when a file under a dirty untracked directory changes")
	}
}

func TestGitDetector_IgnorePathsExcludesLog(t *testing.T) {
	tmpDir := t.TempDir()
	gd, err := NewGitDetector(tmpDir)
	if err != nil {
		t.Fatalf("NewGitDetector failed: %v", err)
	}

	logPath := filepath.Join(tmpDir, "orchestrator_test.log")
	if err := os.WriteFile(logPath, []byte("start\n"), 0644); err != nil {
		t.Fatalf("write log: %v", err)
	}
	gd.IgnorePaths(logPath)
	if err := gd.Reset(); err != nil {
		t.Fatalf("Reset: %v", err)
	}

	if err := os.WriteFile(logPath, []byte("start\niteration done\n"), 0644); err != nil {
		t.Fatalf("append log: %v", err)
	}
	hasProgress, err := gd.Detect()
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if hasProgress {
		t.Fatal("expected no progress when only an ignored log file changes")
	}

	// A real workspace edit should still count.
	if err := gd.Reset(); err != nil {
		t.Fatalf("second Reset: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "real.txt"), []byte("work"), 0644); err != nil {
		t.Fatalf("write real: %v", err)
	}
	hasProgress, err = gd.Detect()
	if err != nil {
		t.Fatalf("Detect after real edit: %v", err)
	}
	if !hasProgress {
		t.Fatal("expected progress for non-ignored workspace edits")
	}
}

func TestGitDetector_RejectsAncestorRepo(t *testing.T) {
	parent := t.TempDir()
	parentGD, err := NewGitDetector(parent)
	if err != nil {
		t.Fatalf("parent NewGitDetector failed: %v", err)
	}
	if !parentGD.isGitRepo() {
		t.Fatal("expected parent temp dir to be a git repo root")
	}

	child := filepath.Join(parent, "subdir")
	if err := os.MkdirAll(child, 0755); err != nil {
		t.Fatalf("mkdir child: %v", err)
	}

	childGD, err := NewGitDetector(child)
	if err != nil {
		t.Fatalf("child NewGitDetector failed: %v", err)
	}
	if !childGD.isGitRepo() {
		t.Fatal("expected child workspace to become its own git root")
	}
	if childGD.workspaceDir == parentGD.workspaceDir {
		t.Fatal("child detector must not reuse the ancestor workspace path")
	}

	// Ancestor-only edits must not register as child progress.
	if err := childGD.Reset(); err != nil {
		t.Fatalf("child Reset: %v", err)
	}
	if err := os.WriteFile(filepath.Join(parent, "outside.txt"), []byte("ancestor"), 0644); err != nil {
		t.Fatalf("write ancestor file: %v", err)
	}
	hasProgress, err := childGD.Detect()
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}
	if hasProgress {
		t.Fatal("expected no progress from ancestor-repo edits outside workspace")
	}
}

func TestGitDetector_Reset(t *testing.T) {
	wd, _ := os.Getwd()
	gd, err := NewGitDetector(wd)
	if err != nil {
		t.Skipf("test skipped: %v", err)
	}

	err = gd.Reset()
	if err != nil {
		t.Fatalf("Reset failed: %v", err)
	}
}

func TestGitDetector_DetailsNoChanges(t *testing.T) {
	tmpDir := t.TempDir()
	gd, err := NewGitDetector(tmpDir)
	if err != nil {
		t.Fatalf("NewGitDetector failed: %v", err)
	}

	// Reset to clean state
	gd.Reset()
	gd.Detect()

	details := gd.Details()
	// With clean state, might be empty or "无变化"
	if details == "" {
		details = "无变化"
	}
	// Just check it doesn't panic
}

func TestGitDetector_DetailsWithChanges(t *testing.T) {
	tmpDir := t.TempDir()
	gd, err := NewGitDetector(tmpDir)
	if err != nil {
		t.Fatalf("NewGitDetector failed: %v", err)
	}

	// Create a new file
	newFile := filepath.Join(tmpDir, "changed.txt")
	os.WriteFile(newFile, []byte("content"), 0644)

	gd.Detect()
	details := gd.Details()

	// Should have some content
	if details == "" {
		t.Error("expected non-empty details with changes")
	}
}

func TestGitDetector_CreateCheckpoint(t *testing.T) {
	tmpDir := t.TempDir()
	gd, err := NewGitDetector(tmpDir)
	if err != nil {
		t.Fatalf("NewGitDetector failed: %v", err)
	}

	// Create a file to commit
	testFile := filepath.Join(tmpDir, "checkpoint.txt")
	os.WriteFile(testFile, []byte("checkpoint content"), 0644)

	err = gd.CreateCheckpoint("test checkpoint")
	if err != nil {
		t.Fatalf("CreateCheckpoint failed: %v", err)
	}
}

func TestGitDetector_QuotedPathWithSpacesIsProgress(t *testing.T) {
	tmpDir := t.TempDir()
	gd, err := NewGitDetector(tmpDir)
	if err != nil {
		t.Fatalf("NewGitDetector failed: %v", err)
	}

	spaced := filepath.Join(tmpDir, "my file.txt")
	if err := os.WriteFile(spaced, []byte("v1"), 0644); err != nil {
		t.Fatalf("create spaced file: %v", err)
	}
	if err := gd.Reset(); err != nil {
		t.Fatalf("Reset: %v", err)
	}

	if err := os.WriteFile(spaced, []byte("v2-spaced-edit"), 0644); err != nil {
		t.Fatalf("edit spaced file: %v", err)
	}
	hasProgress, err := gd.Detect()
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if !hasProgress {
		t.Fatal("expected progress when editing an already-dirty path containing spaces")
	}
}

func TestGitDetector_SymlinkRetargetIsProgress(t *testing.T) {
	tmpDir := t.TempDir()
	gd, err := NewGitDetector(tmpDir)
	if err != nil {
		t.Fatalf("NewGitDetector failed: %v", err)
	}

	a := filepath.Join(tmpDir, "a.txt")
	b := filepath.Join(tmpDir, "b.txt")
	if err := os.WriteFile(a, []byte("same"), 0644); err != nil {
		t.Fatalf("write a: %v", err)
	}
	if err := os.WriteFile(b, []byte("same"), 0644); err != nil {
		t.Fatalf("write b: %v", err)
	}
	link := filepath.Join(tmpDir, "alias")
	if err := os.Symlink(a, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	if err := gd.Reset(); err != nil {
		t.Fatalf("Reset: %v", err)
	}

	if err := os.Remove(link); err != nil {
		t.Fatalf("remove link: %v", err)
	}
	if err := os.Symlink(b, link); err != nil {
		t.Fatalf("retarget link: %v", err)
	}
	hasProgress, err := gd.Detect()
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if !hasProgress {
		t.Fatal("expected progress when an untracked symlink is retargeted")
	}
}

func TestDecodePorcelainPath(t *testing.T) {
	cases := map[string]string{
		`plain.txt`:           "plain.txt",
		`"my file.txt"`:       "my file.txt",
		`"path\\with\\slash"`: `path\with\slash`,
		`"a\tb"`:              "a\tb",
	}
	for in, want := range cases {
		if got := decodePorcelainPath(in); got != want {
			t.Errorf("decodePorcelainPath(%q)=%q, want %q", in, got, want)
		}
	}
}

func TestNormalizePorcelainZ(t *testing.T) {
	raw := "?? my file.txt\x00 M tracked.txt\x00"
	got := normalizePorcelainZ(raw)
	want := "?? my file.txt\n M tracked.txt\n"
	if got != want {
		t.Fatalf("normalizePorcelainZ=%q, want %q", got, want)
	}
}

func TestGitDetector_ManyChangedFiles(t *testing.T) {
	tmpDir := t.TempDir()
	gd, err := NewGitDetector(tmpDir)
	if err != nil {
		t.Fatalf("NewGitDetector failed: %v", err)
	}

	// Create many files to test truncation
	for i := 0; i < 10; i++ {
		fname := filepath.Join(tmpDir, filepath.Base(tmpDir)+string(rune('a'+i))+".txt")
		os.WriteFile(fname, []byte("content"), 0644)
	}

	gd.Detect()
	details := gd.Details()

	// Should have truncation indicator
	if len(details) < 5 {
		t.Errorf("expected more detail output, got %q", details)
	}
}
