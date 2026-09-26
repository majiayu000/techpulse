package progress

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// GitDetector reports changes since Reset, including edits to already-dirty files.
type GitDetector struct {
	workspaceDir  string
	baseline      gitSnapshot
	ignorePaths   map[string]bool
	hasUntracked  bool
	hasModified   bool
	changedFiles  []string
	statusChanged bool
	commitChanged bool
}

type gitFileState struct {
	status  string
	content string
}

type gitSnapshot struct {
	commit string
	files  map[string]gitFileState
}

func NewGitDetector(workspaceDir string) (*GitDetector, error) {
	abs, err := filepath.Abs(workspaceDir)
	if err != nil {
		return nil, fmt.Errorf("解析工作区路径失败: %w", err)
	}
	g := &GitDetector{workspaceDir: abs}
	isRepo, err := g.isGitRepo()
	if err != nil {
		return nil, fmt.Errorf("检查 Git 仓库失败: %w", err)
	}
	if !isRepo {
		if err := g.initRepo(); err != nil {
			return nil, fmt.Errorf("初始化 Git 仓库失败: %w", err)
		}
	}
	if err := g.Reset(); err != nil {
		return nil, fmt.Errorf("获取 Git 初始状态失败: %w", err)
	}
	return g, nil
}

func (g *GitDetector) Name() string { return "git" }

// IgnorePaths excludes operational files such as the active runner log.
func (g *GitDetector) IgnorePaths(paths ...string) {
	if g.ignorePaths == nil {
		g.ignorePaths = make(map[string]bool)
	}
	for _, path := range paths {
		abs, err := filepath.Abs(path)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(g.workspaceDir, abs)
		if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		g.ignorePaths[filepath.ToSlash(rel)] = true
		for existing := range g.baseline.files {
			if g.isIgnored(existing) {
				delete(g.baseline.files, existing)
			}
		}
	}
}

func (g *GitDetector) isIgnored(path string) bool {
	for ignored := range g.ignorePaths {
		if path == ignored || strings.HasPrefix(path, ignored+"/") {
			return true
		}
	}
	return false
}

func (g *GitDetector) Detect() (bool, error) {
	current, err := g.snapshot()
	if err != nil {
		return false, fmt.Errorf("git 检测失败: %w", err)
	}
	g.commitChanged = current.commit != g.baseline.commit && current.commit != ""
	g.statusChanged = false
	g.hasUntracked = false
	g.hasModified = false
	g.changedFiles = g.changedFiles[:0]
	for path, state := range current.files {
		before, present := g.baseline.files[path]
		if !present || before != state {
			g.statusChanged = true
			g.changedFiles = append(g.changedFiles, path)
			if state.status == "??" {
				g.hasUntracked = true
			} else {
				g.hasModified = true
			}
		}
	}
	for path := range g.baseline.files {
		if _, present := current.files[path]; !present {
			g.statusChanged = true
			g.hasModified = true
			g.changedFiles = append(g.changedFiles, path+" (removed)")
		}
	}
	sort.Strings(g.changedFiles)
	if g.commitChanged {
		g.baseline.commit = current.commit
	}
	return g.statusChanged || g.commitChanged, nil
}

func (g *GitDetector) Reset() error {
	snapshot, err := g.snapshot()
	if err != nil {
		return fmt.Errorf("git 重置失败: %w", err)
	}
	g.baseline = snapshot
	g.hasUntracked = false
	g.hasModified = false
	g.changedFiles = nil
	g.statusChanged = false
	g.commitChanged = false
	return nil
}

func (g *GitDetector) Details() string {
	var parts []string
	if g.commitChanged {
		parts = append(parts, "新 commit")
	}
	if g.hasUntracked {
		parts = append(parts, "新文件")
	}
	if g.hasModified {
		parts = append(parts, "已修改")
	}
	if len(g.changedFiles) > 0 {
		limit := len(g.changedFiles)
		if limit > 5 {
			limit = 5
		}
		files := append([]string(nil), g.changedFiles[:limit]...)
		if len(g.changedFiles) > limit {
			files = append(files, fmt.Sprintf("...等 %d 个文件", len(g.changedFiles)))
		}
		parts = append(parts, strings.Join(files, ", "))
	}
	if len(parts) == 0 {
		return "无变化"
	}
	return strings.Join(parts, "; ")
}

func (g *GitDetector) snapshot() (gitSnapshot, error) {
	commit, err := g.getCurrentCommit()
	if err != nil {
		return gitSnapshot{}, err
	}
	states, err := g.getStatusStates()
	if err != nil {
		return gitSnapshot{}, err
	}
	files := make(map[string]gitFileState, len(states))
	for path, status := range states {
		if g.isIgnored(path) {
			continue
		}
		content, err := g.hashWorkspaceFile(path)
		if err != nil {
			return gitSnapshot{}, fmt.Errorf("读取工作区文件 %q 失败: %w", path, err)
		}
		files[path] = gitFileState{status: status, content: content}
	}
	return gitSnapshot{commit: commit, files: files}, nil
}

// -z preserves spaces and non-ASCII paths; -uall lists files inside untracked dirs.
func (g *GitDetector) getStatusStates() (map[string]string, error) {
	out, _, err := g.gitOutputRaw("status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return nil, err
	}
	states := make(map[string]string)
	for pos := 0; pos < len(out); {
		end := strings.IndexByte(out[pos:], 0)
		if end < 0 {
			break
		}
		entry := out[pos : pos+end]
		pos += end + 1
		if len(entry) < 4 {
			continue
		}
		status, path := entry[:2], entry[3:]
		if status[0] == 'R' || status[0] == 'C' || status[1] == 'R' || status[1] == 'C' {
			// In -z output the destination is first; skip the old path.
			if next := strings.IndexByte(out[pos:], 0); next >= 0 {
				pos += next + 1
			}
		}
		states[path] = status
	}
	return states, nil
}

func (g *GitDetector) hashWorkspaceFile(path string) (string, error) {
	full := filepath.Join(g.workspaceDir, filepath.FromSlash(path))
	info, err := os.Lstat(full)
	if errors.Is(err, os.ErrNotExist) {
		return "missing", nil
	}
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(full)
		if err != nil {
			return "", err
		}
		return digestString("symlink:" + target), nil
	}
	if info.IsDir() {
		h := sha256.New()
		err := filepath.WalkDir(full, func(child string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(g.workspaceDir, child)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			if g.isIgnored(rel) {
				return nil
			}
			sum, err := g.hashWorkspaceFile(rel)
			if err != nil {
				return err
			}
			fmt.Fprintf(h, "%s\x00%s\x00", rel, sum)
			return nil
		})
		return hex.EncodeToString(h.Sum(nil)), err
	}
	if !info.Mode().IsRegular() {
		return fmt.Sprintf("special:%v:%d", info.Mode(), info.Size()), nil
	}
	f, err := os.Open(full)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func digestString(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func (g *GitDetector) gitOutputRaw(args ...string) (stdout string, absent bool, outputErr error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = g.workspaceDir
	output, err := cmd.Output()
	if err == nil {
		return string(output), false, nil
	}
	if isGitAbsence(err) {
		return "", true, nil
	}
	return "", false, fmt.Errorf("git %s 失败: %w", strings.Join(args, " "), err)
}

func (g *GitDetector) gitOutput(args ...string) (string, bool, error) {
	out, absent, err := g.gitOutputRaw(args...)
	return strings.TrimSpace(out), absent, err
}

func isGitAbsence(err error) bool {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return false
	}
	msg := string(exitErr.Stderr)
	return strings.Contains(msg, "not a git repository") || strings.Contains(msg, "unknown revision")
}

// isGitRepo checks this workspace rather than accepting an ancestor's repo.
func (g *GitDetector) isGitRepo() (bool, error) {
	top, absent, err := g.gitOutput("rev-parse", "--show-toplevel")
	if err != nil || absent {
		return false, err
	}
	top, err = filepath.Abs(top)
	if err != nil {
		return false, err
	}
	if resolved, err := filepath.EvalSymlinks(top); err == nil {
		top = resolved
	}
	workspace := g.workspaceDir
	if resolved, err := filepath.EvalSymlinks(workspace); err == nil {
		workspace = resolved
	}
	return filepath.Clean(top) == filepath.Clean(workspace), nil
}

func (g *GitDetector) initRepo() error {
	cmd := exec.Command("git", "init")
	cmd.Dir = g.workspaceDir
	if err := cmd.Run(); err != nil {
		return err
	}
	cmd = exec.Command("git", "add", "-A")
	cmd.Dir = g.workspaceDir
	_ = cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "Initial commit by orchestrator", "--allow-empty")
	cmd.Dir = g.workspaceDir
	_ = cmd.Run()
	return nil
}

func (g *GitDetector) getCurrentCommit() (string, error) {
	out, _, err := g.gitOutput("rev-parse", "HEAD")
	return out, err
}

func (g *GitDetector) checkUntracked() (bool, error) {
	out, _, err := g.gitOutput("ls-files", "--others", "--exclude-standard")
	return len(out) > 0, err
}

func (g *GitDetector) checkModified() (bool, error) {
	states, err := g.getStatusStates()
	if err != nil {
		return false, err
	}
	for _, status := range states {
		if status != "??" {
			return true, nil
		}
	}
	return false, nil
}

func (g *GitDetector) getChangedFiles() ([]string, error) {
	states, err := g.getStatusStates()
	if err != nil {
		return nil, err
	}
	files := make([]string, 0, len(states))
	for path := range states {
		files = append(files, path)
	}
	sort.Strings(files)
	return files, nil
}

func (g *GitDetector) CreateCheckpoint(message string) error {
	cmd := exec.Command("git", "add", "-A")
	cmd.Dir = g.workspaceDir
	if err := cmd.Run(); err != nil {
		return err
	}
	cmd = exec.Command("git", "commit", "-m", message, "--allow-empty")
	cmd.Dir = g.workspaceDir
	return cmd.Run()
}
