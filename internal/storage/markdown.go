package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/majiayu000/techpulse/internal/summarizer"
)

// archiveFileLayout is the date layout used for daily archive file names.
const archiveFileLayout = "2006-01-02"

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
//
// The file is written atomically (temp file + rename), so a crash mid-write
// leaves the previous archive intact. After a successful write, archive
// files older than Config.RetentionDays are removed; a non-positive
// RetentionDays disables cleanup. Retention failures are returned as
// errors - never swallowed - even though today's archive was already
// written successfully.
func (s *MarkdownStorage) Save(articles []summarizer.EnrichedArticle) error {
	archiveDir := filepath.Join(s.config.BaseDir, s.config.ArchiveDir)
	if err := os.MkdirAll(archiveDir, 0755); err != nil {
		return fmt.Errorf("create archive dir: %w", err)
	}

	today := time.Now().Format(archiveFileLayout)
	archiveFile := filepath.Join(archiveDir, today+".md")

	content := s.articlesToMarkdown(articles, today)
	if err := writeFileAtomic(archiveFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("write archive: %w", err)
	}

	if err := s.enforceRetention(); err != nil {
		return fmt.Errorf("retention: %w", err)
	}

	return nil
}

// SaveReport writes the daily report to the digest file.
//
// The write is atomic: a failed or interrupted write leaves the previous
// digest completely intact instead of truncating it.
func (s *MarkdownStorage) SaveReport(report summarizer.Report) error {
	if err := os.MkdirAll(s.config.BaseDir, 0755); err != nil {
		return fmt.Errorf("create base dir: %w", err)
	}

	digestPath := filepath.Join(s.config.BaseDir, s.config.DigestFile)
	if err := writeFileAtomic(digestPath, []byte(report.Content), 0644); err != nil {
		return fmt.Errorf("write digest: %w", err)
	}

	return nil
}

// writeFileAtomic writes data to path atomically. It writes to a temporary
// file in the same directory as the final destination (same filesystem, so
// the rename below cannot fall back to a copy), flushes it to stable
// storage, then renames it over the destination. Readers therefore see
// either the complete previous content or the complete new content - never
// a torn write. On any error, the destination is left untouched and the
// temporary file is cleaned up.
//
// Symlinks at path are preserved: the write resolves to the target and
// replaces that file, matching os.WriteFile behavior. Permissions follow
// the previous destination mode when it exists; for new files, OpenFile
// applies the process umask to perm so restrictive umasks are honored.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dest, err := resolveWritePath(path)
	if err != nil {
		return err
	}
	dir, base := filepath.Dir(dest), filepath.Base(dest)

	mode := perm
	if fi, err := os.Stat(dest); err == nil {
		mode = fi.Mode().Perm()
	}

	tmp, err := os.CreateTemp(dir, "."+base+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	// Best-effort cleanup: a no-op once the rename below has consumed the
	// temp file (it no longer exists, so Remove reports IsNotExist).
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	// CreateTemp uses 0600. For an existing destination, restore its mode.
	// For a new file, re-create the mode through OpenFile semantics by
	// chmod'ing to the umask-adjusted value of the requested perm.
	if _, err := os.Stat(dest); err == nil {
		if err := os.Chmod(tmpName, mode); err != nil {
			return fmt.Errorf("chmod temp file: %w", err)
		}
	} else {
		if err := chmodWithUmask(tmpName, perm); err != nil {
			return err
		}
	}

	if err := os.Rename(tmpName, dest); err != nil {
		return fmt.Errorf("replace %s: %w", base, err)
	}

	return nil
}

// resolveWritePath returns the filesystem path that should receive an
// atomic replace. Regular files and missing paths are returned as-is; a
// symlink is resolved so the write updates the target and leaves the
// symlink directory entry intact.
func resolveWritePath(path string) (string, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return path, nil
		}
		return "", fmt.Errorf("stat %s: %w", path, err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		return path, nil
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("resolve symlink %s: %w", path, err)
	}
	return resolved, nil
}

// chmodWithUmask sets path's mode to perm after applying the process umask,
// matching os.OpenFile / os.WriteFile creation behavior for new files.
func chmodWithUmask(path string, perm os.FileMode) error {
	probe, err := os.CreateTemp(filepath.Dir(path), ".perm-probe-*")
	if err != nil {
		return fmt.Errorf("create perm probe: %w", err)
	}
	probeName := probe.Name()
	probe.Close()
	defer os.Remove(probeName)

	// Recreate with the requested permission bits so the kernel applies umask.
	os.Remove(probeName)
	f, err := os.OpenFile(probeName, os.O_RDWR|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return fmt.Errorf("create perm probe: %w", err)
	}
	fi, err := f.Stat()
	f.Close()
	if err != nil {
		return fmt.Errorf("stat perm probe: %w", err)
	}
	if err := os.Chmod(path, fi.Mode().Perm()); err != nil {
		return fmt.Errorf("chmod temp file: %w", err)
	}
	return nil
}

// enforceRetention removes daily archive files strictly older than
// Config.RetentionDays, judged by the date encoded in their file names
// (<yyyy-mm-dd>.md). A non-positive RetentionDays disables retention
// entirely. Only file names that parse as dated archives are ever removed;
// anything else found in the archive directory is left untouched. Every
// deletion failure is reported through the returned error.
func (s *MarkdownStorage) enforceRetention() error {
	if s.config.RetentionDays <= 0 {
		return nil
	}

	archiveDir := filepath.Join(s.config.BaseDir, s.config.ArchiveDir)
	entries, err := os.ReadDir(archiveDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("list archives: %w", err)
	}

	now := time.Now()
	cutoff := time.Date(now.Year(), now.Month(), now.Day(),
		0, 0, 0, 0, now.Location()).AddDate(0, 0, -s.config.RetentionDays)

	var delErrs []error
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".md") {
			continue
		}
		fileDate, parseErr := time.ParseInLocation(
			archiveFileLayout, strings.TrimSuffix(name, ".md"), now.Location())
		if parseErr != nil {
			// Not a dated archive file; never delete unrecognized files.
			continue
		}
		if !fileDate.Before(cutoff) {
			// Still within the retention window.
			continue
		}
		if rmErr := os.Remove(filepath.Join(archiveDir, name)); rmErr != nil {
			delErrs = append(delErrs, fmt.Errorf("remove %s: %w", name, rmErr))
		}
	}

	return errors.Join(delErrs...)
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

	// Order sections deterministically: by each source's highest article
	// importance descending (mirroring the report generator's
	// importance-first ordering), ties broken by source name ascending.
	sources := make([]string, 0, len(bySource))
	topImportance := make(map[string]float64, len(bySource))
	for source, sourceArticles := range bySource {
		sources = append(sources, source)
		top := sourceArticles[0].Importance
		for _, a := range sourceArticles[1:] {
			if a.Importance > top {
				top = a.Importance
			}
		}
		topImportance[source] = top
	}
	sort.Slice(sources, func(i, j int) bool {
		if topImportance[sources[i]] != topImportance[sources[j]] {
			return topImportance[sources[i]] > topImportance[sources[j]]
		}
		return sources[i] < sources[j]
	})

	for _, source := range sources {
		sourceArticles := sortByImportanceDescTitle(bySource[source])
		md += fmt.Sprintf("## %s (%d articles)\n\n", source, len(sourceArticles))

		for _, a := range sourceArticles {
			md += fmt.Sprintf("### %s\n", summarizer.MarkdownLink(a.Title, a.URL))
			md += fmt.Sprintf("- Score: %d | Comments: %d | Importance: %.1f/10\n",
				a.Score, a.Comments, a.Importance)
			if len(a.MatchedKeywords) > 0 {
				md += fmt.Sprintf("- Keywords: %v\n", a.MatchedKeywords)
			}
			if a.Author != "" {
				md += fmt.Sprintf("- Author: %s\n", summarizer.SanitizeMarkdownText(a.Author))
			}
			md += "\n"
		}
	}

	md += fmt.Sprintf("\n---\nGenerated at: %s\n", time.Now().Format(time.RFC3339))
	return md
}

// sortByImportanceDescTitle returns a copy of articles ordered by
// importance descending with title ascending as tiebreaker, matching the
// report generator's importance-first ordering while remaining fully
// deterministic.
func sortByImportanceDescTitle(articles []summarizer.EnrichedArticle) []summarizer.EnrichedArticle {
	sorted := make([]summarizer.EnrichedArticle, len(articles))
	copy(sorted, articles)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Importance != sorted[j].Importance {
			return sorted[i].Importance > sorted[j].Importance
		}
		return sorted[i].Title < sorted[j].Title
	})
	return sorted
}
