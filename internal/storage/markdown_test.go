package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/majiayu000/techpulse/internal/collector"
	"github.com/majiayu000/techpulse/internal/filter"
	"github.com/majiayu000/techpulse/internal/summarizer"
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

// --- atomic writes ---

// tempResidue returns leftover hidden temp files (from interrupted atomic
// writes) in dir. An empty result means the atomic writer cleaned up.
func tempResidue(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir %s: %v", dir, err)
	}
	var found []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") && strings.Contains(e.Name(), ".tmp-") {
			found = append(found, e.Name())
		}
	}
	return found
}

func TestWriteFileAtomicCreatesAndReplaces(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.md")

	if err := writeFileAtomic(path, []byte("version-1"), 0644); err != nil {
		t.Fatalf("writeFileAtomic failed: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(got) != "version-1" {
		t.Errorf("expected 'version-1', got '%s'", got)
	}

	// Replacing an existing file publishes the complete new content while
	// preserving the destination's existing permissions.
	if err := os.Chmod(path, 0600); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	if err := writeFileAtomic(path, []byte("version-2"), 0644); err != nil {
		t.Fatalf("writeFileAtomic replace failed: %v", err)
	}

	got, err = os.ReadFile(path)
	if err != nil {
		t.Fatalf("read replaced file: %v", err)
	}
	if string(got) != "version-2" {
		t.Errorf("expected 'version-2', got '%s'", got)
	}
	if fi, err := os.Stat(path); err != nil {
		t.Fatalf("stat: %v", err)
	} else if fi.Mode().Perm() != 0600 {
		t.Errorf("expected preserved perm 0600, got %v", fi.Mode().Perm())
	}

	if residue := tempResidue(t, dir); len(residue) > 0 {
		t.Errorf("temp files left behind after success: %v", residue)
	}
}

func TestWriteFileAtomicPreservesSymlinkTarget(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "real-digest.md")
	link := filepath.Join(dir, "DIGEST.md")
	if err := os.WriteFile(target, []byte("old"), 0600); err != nil {
		t.Fatalf("seed target: %v", err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	if err := writeFileAtomic(link, []byte("new-content"), 0644); err != nil {
		t.Fatalf("writeFileAtomic through symlink: %v", err)
	}

	fi, err := os.Lstat(link)
	if err != nil {
		t.Fatalf("lstat link: %v", err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Fatal("symlink directory entry was replaced by a regular file")
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if string(got) != "new-content" {
		t.Errorf("target content = %q, want new-content", got)
	}
	if fi, err := os.Stat(target); err != nil {
		t.Fatalf("stat target: %v", err)
	} else if fi.Mode().Perm() != 0600 {
		t.Errorf("target perm = %v, want preserved 0600", fi.Mode().Perm())
	}
}

func TestWriteFileAtomicCreatesThroughDanglingSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "real-digest.md") // not created yet
	link := filepath.Join(dir, "DIGEST.md")
	// Relative link, matching common publish layouts.
	if err := os.Symlink("real-digest.md", link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	if err := writeFileAtomic(link, []byte("created-via-link"), 0644); err != nil {
		t.Fatalf("writeFileAtomic through dangling symlink: %v", err)
	}

	fi, err := os.Lstat(link)
	if err != nil {
		t.Fatalf("lstat link: %v", err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Fatal("symlink directory entry was replaced by a regular file")
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read created target: %v", err)
	}
	if string(got) != "created-via-link" {
		t.Errorf("target content = %q, want created-via-link", got)
	}
}

func TestSaveReportFailurePreservesDigest(t *testing.T) {
	goodContent := "# Good Digest\n\nPrevious good digest."

	newReport := summarizer.Report{
		Title:   "New Report",
		Date:    "2026-01-02",
		Content: "# New Report\n\nThis write must fail.",
	}

	cases := []struct {
		name string
		// digestFile is relative to the base directory. A pre-existing good
		// digest can only be seeded when its parent directory exists, so
		// wantPreservedFile is false for cases whose target location cannot
		// hold a file in the first place.
		digestFile        string
		makeFailing       func(t *testing.T, baseDir string)
		wantPreservedFile bool
		wantDigestDirGone bool
	}{
		{
			name:       "unwritable base directory",
			digestFile: "DIGEST.md",
			makeFailing: func(t *testing.T, baseDir string) {
				if os.Getuid() == 0 {
					t.Skip("permission-based failure injection is unreliable when running as root")
				}
				// r-x: reading the previous digest works, creating files does not.
				if err := os.Chmod(baseDir, 0500); err != nil {
					t.Fatalf("chmod base dir: %v", err)
				}
				t.Cleanup(func() { os.Chmod(baseDir, 0755) })
			},
			wantPreservedFile: true,
		},
		{
			name:       "digest path is a directory blocks the rename",
			digestFile: "DIGEST.md",
			makeFailing: func(t *testing.T, baseDir string) {
				// A directory at the digest path makes the final rename fail
				// after the temp file has already been written; the temp file
				// must be cleaned up and the directory left untouched.
				if err := os.Mkdir(filepath.Join(baseDir, "DIGEST.md"), 0755); err != nil {
					t.Fatalf("mkdir digest path: %v", err)
				}
			},
			wantDigestDirGone: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			config := Config{BaseDir: tmpDir, DigestFile: tc.digestFile}
			storage := NewMarkdownStorage(config)

			if tc.wantPreservedFile {
				// Establish a known-good digest first.
				if err := storage.SaveReport(summarizer.Report{Content: goodContent}); err != nil {
					t.Fatalf("seeding good digest failed: %v", err)
				}
			}

			tc.makeFailing(t, tmpDir)

			err := storage.SaveReport(newReport)
			if err == nil {
				t.Fatal("expected SaveReport to return an error on failed write")
			}

			digestPath := filepath.Join(tmpDir, tc.digestFile)
			if tc.wantPreservedFile {
				// The previous digest must survive byte-for-byte.
				got, readErr := os.ReadFile(digestPath)
				if readErr != nil {
					t.Fatalf("previous digest unreadable after failed write: %v", readErr)
				}
				if string(got) != goodContent {
					t.Errorf("previous digest was damaged:\nwant %q\ngot  %q", goodContent, string(got))
				}
			}
			if tc.wantDigestDirGone {
				if fi, statErr := os.Stat(digestPath); statErr != nil || !fi.IsDir() {
					t.Fatalf("directory at digest path should remain untouched, stat err: %v", statErr)
				}
			}

			if residue := tempResidue(t, tmpDir); len(residue) > 0 {
				t.Errorf("temp files left behind after failure: %v", residue)
			}
		})
	}
}

func TestSaveArchiveFailurePreservesPreviousArchive(t *testing.T) {
	tmpDir := t.TempDir()
	config := Config{
		BaseDir:       tmpDir,
		ArchiveDir:    "archive",
		RetentionDays: 30,
	}
	storage := NewMarkdownStorage(config)

	oldArticles := []summarizer.EnrichedArticle{
		createTestEnrichedArticle("1", "Old Article", "hackernews", 5.0),
	}
	if err := storage.Save(oldArticles); err != nil {
		t.Fatalf("seeding archive failed: %v", err)
	}

	today := time.Now().Format(archiveFileLayout)
	archiveFile := filepath.Join(tmpDir, "archive", today+".md")
	before, err := os.ReadFile(archiveFile)
	if err != nil {
		t.Fatalf("read seeded archive: %v", err)
	}

	if os.Getuid() == 0 {
		t.Skip("permission-based failure injection is unreliable when running as root")
	}
	archiveDir := filepath.Join(tmpDir, "archive")
	// r-x: listing works, creating the temp file does not.
	if err := os.Chmod(archiveDir, 0500); err != nil {
		t.Fatalf("chmod archive dir: %v", err)
	}
	t.Cleanup(func() { os.Chmod(archiveDir, 0755) })

	err = storage.Save([]summarizer.EnrichedArticle{
		createTestEnrichedArticle("2", "New Article", "rss", 6.0),
	})
	if err == nil {
		t.Fatal("expected Save to return an error on failed write")
	}

	after, err := os.ReadFile(archiveFile)
	if err != nil {
		t.Fatalf("previous archive unreadable after failed write: %v", err)
	}
	if string(before) != string(after) {
		t.Error("previous archive was modified by a failed save")
	}
}

// --- deterministic section ordering ---

// stablePart strips the trailing generated-at timestamp so repeated
// conversions of the same input can be compared byte-for-byte.
func stablePart(markdown string) string {
	if i := strings.Index(markdown, "\n---\n"); i >= 0 {
		return markdown[:i]
	}
	return markdown
}

func TestArticlesToMarkdownSectionOrderDeterministic(t *testing.T) {
	storage := NewMarkdownStorage(Config{BaseDir: t.TempDir()})

	articles := []summarizer.EnrichedArticle{
		createTestEnrichedArticle("1", "RSS Article", "rss", 8.0),
		createTestEnrichedArticle("2", "Blog Low", "blog", 6.0),
		createTestEnrichedArticle("3", "HN Article", "hackernews", 9.5),
		createTestEnrichedArticle("4", "Blog High", "blog", 7.9),
	}

	// Sections ordered by each source's highest article importance desc:
	// hackernews (9.5), rss (8.0), blog (7.9).
	wantHeaders := []string{"## hackernews", "## rss", "## blog"}

	var wantStable string
	for i := 0; i < 30; i++ {
		got := storage.articlesToMarkdown(articles, "2026-08-23")
		stable := stablePart(got)

		if i == 0 {
			wantStable = stable
			last := -1
			for _, h := range wantHeaders {
				idx := strings.Index(stable, h)
				if idx < 0 {
					t.Fatalf("output missing section header %q:\n%s", h, stable)
				}
				if idx < last {
					t.Fatalf("sections out of order: %q appears after an earlier header:\n%s", h, stable)
				}
				last = idx
			}
			continue
		}
		if stable != wantStable {
			t.Fatalf("conversion output is unstable across calls (iteration %d):\nfirst:\n%s\nthen:\n%s",
				i, wantStable, stable)
		}
	}
}

func TestArticlesToMarkdownOrdersArticlesWithinSection(t *testing.T) {
	storage := NewMarkdownStorage(Config{BaseDir: t.TempDir()})

	articles := []summarizer.EnrichedArticle{
		createTestEnrichedArticle("1", "Zeta Low", "src", 5.0),
		createTestEnrichedArticle("2", "Mid High", "src", 9.0),
		createTestEnrichedArticle("3", "Alpha High", "src", 9.0), // ties with Mid -> title asc wins
	}

	got := stablePart(storage.articlesToMarkdown(articles, "2026-08-23"))

	alpha := strings.Index(got, "### [Alpha High]")
	mid := strings.Index(got, "### [Mid High]")
	zeta := strings.Index(got, "### [Zeta Low]")

	if alpha < 0 || mid < 0 || zeta < 0 {
		t.Fatalf("missing expected article headings:\n%s", got)
	}
	if !(alpha < mid && mid < zeta) {
		t.Errorf("expected Alpha High < Mid High < Zeta Low (importance desc, title asc), got order:\n%s", got)
	}
}

// --- retention ---

// dateDaysAgo formats a date string the given number of days in the past.
func dateDaysAgo(days int) string {
	return time.Now().AddDate(0, 0, -days).Format(archiveFileLayout)
}

func writeArchiveFile(t *testing.T, archiveDir, date, content string) {
	t.Helper()
	path := filepath.Join(archiveDir, date+".md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("create archive fixture %s: %v", path, err)
	}
}

func TestEnforceRetention(t *testing.T) {
	cases := []struct {
		name          string
		retentionDays int
		fixtures      map[string]string // date part (without .md) -> content
		wantKept      []string
		wantRemoved   []string
	}{
		{
			name:          "disabled when retention days is zero",
			retentionDays: 0,
			fixtures:      map[string]string{dateDaysAgo(90): "ancient"},
			wantKept:      []string{dateDaysAgo(90)},
		},
		{
			name:          "disabled when retention days is negative",
			retentionDays: -1,
			fixtures:      map[string]string{dateDaysAgo(90): "ancient"},
			wantKept:      []string{dateDaysAgo(90)},
		},
		{
			name:          "removes archives strictly older than window",
			retentionDays: 7,
			fixtures: map[string]string{
				dateDaysAgo(0):  "today",
				dateDaysAgo(1):  "yesterday",
				dateDaysAgo(8):  "eight days old",
				dateDaysAgo(40): "forty days old",
			},
			wantKept:    []string{dateDaysAgo(0), dateDaysAgo(1)},
			wantRemoved: []string{dateDaysAgo(8), dateDaysAgo(40)},
		},
		{
			name:          "keeps archive exactly at cutoff boundary",
			retentionDays: 7,
			fixtures: map[string]string{
				dateDaysAgo(7): "exactly at cutoff",
				dateDaysAgo(8): "one day past cutoff",
			},
			wantKept:    []string{dateDaysAgo(7)},
			wantRemoved: []string{dateDaysAgo(8)},
		},
		{
			name:          "leaves unrecognized files and directories untouched",
			retentionDays: 7,
			fixtures: map[string]string{
				"not-a-date":    "looks like an archive but is not dated",
				dateDaysAgo(90): "genuinely old archive",
			},
			wantKept:    []string{"not-a-date"},
			wantRemoved: []string{dateDaysAgo(90)},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			archiveDir := filepath.Join(tmpDir, "archive")
			if err := os.MkdirAll(archiveDir, 0755); err != nil {
				t.Fatalf("mkdir archive: %v", err)
			}
			for date, content := range tc.fixtures {
				writeArchiveFile(t, archiveDir, date, content)
			}

			// Entries the retention logic must never touch, present in every case.
			readme := filepath.Join(archiveDir, "README.txt")
			if err := os.WriteFile(readme, []byte("keep me"), 0644); err != nil {
				t.Fatalf("write README.txt: %v", err)
			}
			likeArchiveDir := filepath.Join(archiveDir, "2019-01-01.md")
			if err := os.Mkdir(likeArchiveDir, 0755); err != nil {
				t.Fatalf("mkdir archive-named dir: %v", err)
			}

			storage := NewMarkdownStorage(Config{
				BaseDir:       tmpDir,
				ArchiveDir:    "archive",
				RetentionDays: tc.retentionDays,
			})

			if err := storage.enforceRetention(); err != nil {
				t.Fatalf("enforceRetention returned error: %v", err)
			}

			for _, date := range tc.wantKept {
				path := filepath.Join(archiveDir, date+".md")
				if _, err := os.Stat(path); err != nil {
					t.Errorf("expected %s.md to be kept, but it is missing (%v)", date, err)
				}
			}
			for _, date := range tc.wantRemoved {
				path := filepath.Join(archiveDir, date+".md")
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Errorf("expected %s.md to be removed, but it still exists", date)
				}
			}

			if _, err := os.Stat(readme); err != nil {
				t.Error("README.txt must never be removed by retention")
			}
			if _, err := os.Stat(likeArchiveDir); err != nil {
				t.Error("directories must never be removed by retention")
			}
		})
	}
}

func TestSaveRunsRetentionAfterWrite(t *testing.T) {
	tmpDir := t.TempDir()
	config := Config{
		BaseDir:       tmpDir,
		ArchiveDir:    "archive",
		DigestFile:    "DIGEST.md",
		RetentionDays: 5,
	}
	storage := NewMarkdownStorage(config)

	archiveDir := filepath.Join(tmpDir, "archive")
	if err := os.MkdirAll(archiveDir, 0755); err != nil {
		t.Fatalf("mkdir archive: %v", err)
	}
	expiredDate := dateDaysAgo(10)
	writeArchiveFile(t, archiveDir, expiredDate, "expired")

	articles := []summarizer.EnrichedArticle{
		createTestEnrichedArticle("1", "Today Article", "hackernews", 8.0),
	}
	if err := storage.Save(articles); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	today := dateDaysAgo(0)
	if _, err := os.Stat(filepath.Join(tmpDir, "archive", today+".md")); err != nil {
		t.Errorf("today's archive missing after Save: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "archive", expiredDate+".md")); !os.IsNotExist(err) {
		t.Error("archive older than RetentionDays should have been removed by Save")
	}
}

func TestEnforceRetentionReportsDeletionErrors(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("permission-based failure injection is unreliable when running as root")
	}

	tmpDir := t.TempDir()
	archiveDir := filepath.Join(tmpDir, "archive")
	if err := os.MkdirAll(archiveDir, 0755); err != nil {
		t.Fatalf("mkdir archive: %v", err)
	}
	expiredDate := dateDaysAgo(30)
	writeArchiveFile(t, archiveDir, expiredDate, "cannot be deleted")

	storage := NewMarkdownStorage(Config{
		BaseDir:       tmpDir,
		ArchiveDir:    "archive",
		RetentionDays: 7,
	})

	// r-x: entries are visible but removal fails.
	if err := os.Chmod(archiveDir, 0500); err != nil {
		t.Fatalf("chmod archive dir: %v", err)
	}
	t.Cleanup(func() { os.Chmod(archiveDir, 0755) })

	err := storage.enforceRetention()
	if err == nil {
		t.Fatal("expected enforceRetention to report the deletion failure")
	}
	if !strings.Contains(err.Error(), expiredDate+".md") {
		t.Errorf("deletion error should name the failing file %s.md, got: %v", expiredDate, err)
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
