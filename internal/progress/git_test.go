package progress

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestGitDetector_NewInGitRepo(t *testing.T) {
	gd, err := NewGitDetector(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if gd == nil {
		t.Fatal("expected non-nil GitDetector")
	}
}

func TestGitDetector_Name(t *testing.T) {
	gd, err := NewGitDetector(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	if gd.Name() != "git" {
		t.Errorf("expected name 'git', got %q", gd.Name())
	}
}

func TestGitDetector_DetectNoChanges(t *testing.T) {
	gd, err := NewGitDetector(t.TempDir())
	if err != nil {
		t.Fatal(err)
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

func TestGitDetector_Reset(t *testing.T) {
	gd, err := NewGitDetector(t.TempDir())
	if err != nil {
		t.Fatal(err)
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

// TestGitDetector_DetailsDoesNotMutateChangedFiles locks in that Details()
// truncates a COPY of changedFiles; appending the "...等 N 个文件" marker
// must never overwrite names in the shared backing array.
func TestGitDetector_DetailsDoesNotMutateChangedFiles(t *testing.T) {
	tests := []struct {
		name         string
		files        []string
		wantContains []string
		wantMissing  []string
	}{
		{
			name:  "empty list",
			files: nil,
		},
		{
			name:         "single file",
			files:        []string{"a.txt"},
			wantContains: []string{"a.txt"},
		},
		{
			name:         "exactly five files",
			files:        []string{"f1", "f2", "f3", "f4", "f5"},
			wantContains: []string{"f1, f2, f3, f4, f5"},
			wantMissing:  []string{"...等"},
		},
		{
			// Regression: len==6 made append(files[:5], ...) write into
			// the shared backing array at index 5.
			name:         "six files aliasing case",
			files:        []string{"f1", "f2", "f3", "f4", "f5", "f6"},
			wantContains: []string{"f1, f2, f3, f4, f5", "...等 6 个文件"},
			wantMissing:  []string{"f6,"},
		},
		{
			name:         "ten files",
			files:        []string{"f1", "f2", "f3", "f4", "f5", "f6", "f7", "f8", "f9", "f10"},
			wantContains: []string{"f1, f2, f3, f4, f5, ...等 10 个文件"},
			wantMissing:  []string{"f6"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snapshot := append([]string(nil), tt.files...)
			g := &GitDetector{changedFiles: tt.files}

			details := g.Details()

			if !reflect.DeepEqual(tt.files, snapshot) {
				t.Errorf("Details() mutated changedFiles: before %v, after %v", snapshot, tt.files)
			}
			for _, sub := range tt.wantContains {
				if !strings.Contains(details, sub) {
					t.Errorf("expected details to contain %q, got %q", sub, details)
				}
			}
			for _, sub := range tt.wantMissing {
				if strings.Contains(details, sub) {
					t.Errorf("expected details NOT to contain %q, got %q", sub, details)
				}
			}
		})
	}
}

// TestGitDetector_DetectSurfacesExecErrors verifies Detect distinguishes
// real exec failures (surfaced as error) from legitimate "no data" states
// such as a directory that is not a git repository.
func TestGitDetector_DetectSurfacesExecErrors(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T) string
		wantErr bool
	}{
		{
			name: "workspace path is a regular file",
			setup: func(t *testing.T) string {
				tmp := t.TempDir()
				file := filepath.Join(tmp, "not-a-dir")
				if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
					t.Fatal(err)
				}
				return file
			},
			wantErr: true,
		},
		{
			name: "workspace dir does not exist",
			setup: func(t *testing.T) string {
				return filepath.Join(t.TempDir(), "missing-dir")
			},
			wantErr: true,
		},
		{
			name: "plain directory without git repo is legitimate absence",
			setup: func(t *testing.T) string {
				return t.TempDir()
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &GitDetector{workspaceDir: tt.setup(t)}

			hasProgress, err := g.Detect()
			if tt.wantErr && err == nil {
				t.Errorf("expected Detect to surface an exec error, got hasProgress=%v err=nil", hasProgress)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("expected no error for legitimate absence, got %v", err)
			}
			if !tt.wantErr && hasProgress {
				t.Error("expected no progress for a workspace without git data")
			}

			if _, err := g.checkUntracked(); (err != nil) != tt.wantErr {
				t.Errorf("checkUntracked() err = %v, wantErr = %v", err, tt.wantErr)
			}
			if _, err := g.checkModified(); (err != nil) != tt.wantErr {
				t.Errorf("checkModified() err = %v, wantErr = %v", err, tt.wantErr)
			}
			if _, err := g.getCurrentCommit(); (err != nil) != tt.wantErr {
				t.Errorf("getCurrentCommit() err = %v, wantErr = %v", err, tt.wantErr)
			}
			if _, err := g.getChangedFiles(); (err != nil) != tt.wantErr {
				t.Errorf("getChangedFiles() err = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

// TestGitDetector_NewSurfacesExecError verifies the constructor fails closed
// when the workspace is unusable instead of silently treating it as
// "not a repo" and attempting init.
func TestGitDetector_NewSurfacesExecError(t *testing.T) {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "not-a-dir")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := NewGitDetector(file); err == nil {
		t.Error("expected NewGitDetector to return an error for an unusable workspace")
	}
}

// TestGitDetector_EmptyRepoNoCommits verifies a repo with no commits is
// treated as legitimate "no HEAD yet" rather than an exec failure.
func TestGitDetector_EmptyRepoNoCommits(t *testing.T) {
	tmp := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = tmp
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v: %s", args, err, out)
		}
	}
	run("init")

	g := &GitDetector{workspaceDir: tmp}

	commit, err := g.getCurrentCommit()
	if err != nil {
		t.Fatalf("expected empty commit for repo without commits, got error %v", err)
	}
	if commit != "" {
		t.Errorf("expected empty commit hash, got %q", commit)
	}

	if _, err := g.Detect(); err != nil {
		t.Errorf("expected Detect to treat empty repo as absence, got %v", err)
	}
}

func TestGitDetector_DirtyBaselineAndContentEdit(t *testing.T) {
	dir := t.TempDir()
	g, err := NewGitDetector(dir)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "already dirty.txt")
	if err := os.WriteFile(file, []byte("before"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := g.Reset(); err != nil {
		t.Fatal(err)
	}
	if changed, err := g.Detect(); err != nil || changed {
		t.Fatalf("unchanged dirty file reported progress: changed=%v err=%v", changed, err)
	}
	if err := os.WriteFile(file, []byte("after"), 0o644); err != nil {
		t.Fatal(err)
	}
	if changed, err := g.Detect(); err != nil || !changed {
		t.Fatalf("content edit not reported: changed=%v err=%v", changed, err)
	}
	if err := g.Reset(); err != nil {
		t.Fatal(err)
	}
	if changed, err := g.Detect(); err != nil || changed {
		t.Fatalf("post-reset dirty file reported progress: changed=%v err=%v", changed, err)
	}
}

func TestGitDetector_IgnoreOperationalLog(t *testing.T) {
	dir := t.TempDir()
	g, err := NewGitDetector(dir)
	if err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(dir, "runner.log")
	if err := os.WriteFile(logPath, []byte("start"), 0o644); err != nil {
		t.Fatal(err)
	}
	g.IgnorePaths(logPath)
	if err := g.Reset(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(logPath, []byte("start\niteration done"), 0o644); err != nil {
		t.Fatal(err)
	}
	if changed, err := g.Detect(); err != nil || changed {
		t.Fatalf("log-only write reported progress: changed=%v err=%v", changed, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "work.txt"), []byte("work"), 0o644); err != nil {
		t.Fatal(err)
	}
	if changed, err := g.Detect(); err != nil || !changed {
		t.Fatalf("workspace edit not reported: changed=%v err=%v", changed, err)
	}
}

func TestGitDetector_DoesNotUseAncestorRepository(t *testing.T) {
	parent := t.TempDir()
	if _, err := NewGitDetector(parent); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(parent, "child")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}
	g, err := NewGitDetector(child)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(child, ".git")); err != nil {
		t.Fatalf("child workspace did not get its own Git repository: %v", err)
	}
	if err := g.Reset(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(parent, "outside.txt"), []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}
	if changed, err := g.Detect(); err != nil || changed {
		t.Fatalf("ancestor edit reported as child progress: changed=%v err=%v", changed, err)
	}
}
