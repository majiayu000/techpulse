package progress

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// GitDetector 基于 Git 检测代码变化
type GitDetector struct {
	workspaceDir    string
	lastCommit      string
	baselineStatus  string // porcelain snapshot taken at Reset
	baselineContent string // content fingerprint of dirty paths at Reset
	hasUntracked    bool
	hasModified     bool
	changedFiles    []string
	statusChanged   bool
	contentChanged  bool
	commitChanged   bool
	ignorePaths     map[string]bool // workspace-relative paths excluded from progress
}

// NewGitDetector 创建 Git 检测器
func NewGitDetector(workspaceDir string) (*GitDetector, error) {
	abs, err := filepath.Abs(workspaceDir)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace dir: %w", err)
	}
	g := &GitDetector{workspaceDir: abs}

	// 检查是否是 Git 仓库（必须是 workspace 自身为仓库根，不能是祖先仓库）
	if !g.isGitRepo() {
		// 初始化 Git 仓库
		if err := g.initRepo(); err != nil {
			return nil, fmt.Errorf("初始化 Git 仓库失败: %w", err)
		}
	}

	// 获取当前 commit 与初始状态快照
	g.lastCommit = g.getCurrentCommit()
	g.baselineStatus = g.getStatusPorcelain()
	g.baselineContent = g.getContentFingerprint(g.baselineStatus)

	return g, nil
}

func (g *GitDetector) Name() string {
	return "git"
}

// IgnorePaths excludes workspace-relative operational files (e.g. the active
// orchestrator log) from status/content fingerprints so they do not reset
// consecutive_no_progress. Paths outside the workspace are ignored.
func (g *GitDetector) IgnorePaths(paths ...string) {
	if g.ignorePaths == nil {
		g.ignorePaths = make(map[string]bool)
	}
	for _, p := range paths {
		abs, err := filepath.Abs(p)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(g.workspaceDir, abs)
		if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
			continue
		}
		g.ignorePaths[filepath.ToSlash(rel)] = true
	}
	// Refresh baseline so ignored paths are dropped immediately.
	g.baselineStatus = g.getStatusPorcelain()
	g.baselineContent = g.getContentFingerprint(g.baselineStatus)
}

func (g *GitDetector) Detect() (bool, error) {
	currentStatus := g.getStatusPorcelain()
	currentCommit := g.getCurrentCommit()
	currentContent := g.getContentFingerprint(currentStatus)

	g.commitChanged = currentCommit != g.lastCommit && currentCommit != ""
	g.statusChanged = currentStatus != g.baselineStatus
	g.contentChanged = currentContent != g.baselineContent

	g.hasUntracked = g.checkUntracked()
	g.hasModified = g.checkModified()
	g.changedFiles = g.diffStatusFiles(g.baselineStatus, currentStatus)
	if g.contentChanged && !g.statusChanged {
		// Porcelain unchanged but file contents edited — surface dirty paths.
		for path := range porcelainFiles(currentStatus) {
			g.changedFiles = append(g.changedFiles, path)
		}
	}

	hasProgress := g.statusChanged || g.contentChanged || g.commitChanged

	if g.commitChanged {
		g.lastCommit = currentCommit
	}

	return hasProgress, nil
}

func (g *GitDetector) Reset() error {
	g.lastCommit = g.getCurrentCommit()
	g.baselineStatus = g.getStatusPorcelain()
	g.baselineContent = g.getContentFingerprint(g.baselineStatus)
	g.hasUntracked = false
	g.hasModified = false
	g.changedFiles = nil
	g.statusChanged = false
	g.contentChanged = false
	g.commitChanged = false
	return nil
}

func (g *GitDetector) Details() string {
	var parts []string
	if g.commitChanged {
		parts = append(parts, "新 commit")
	}
	if g.statusChanged || g.contentChanged {
		if g.hasUntracked {
			parts = append(parts, "新文件")
		}
		if g.hasModified || g.contentChanged {
			parts = append(parts, "已修改")
		}
	}
	if len(g.changedFiles) > 0 {
		files := g.changedFiles
		if len(files) > 5 {
			files = append(files[:5], fmt.Sprintf("...等 %d 个文件", len(g.changedFiles)))
		}
		parts = append(parts, strings.Join(files, ", "))
	}
	if len(parts) == 0 {
		return "无变化"
	}
	return strings.Join(parts, "; ")
}

// isGitRepo 检查 workspace 自身是否为 Git 仓库根目录。
// 祖先目录中的仓库不算，避免把外层项目的变更当成 worker 进展。
func (g *GitDetector) isGitRepo() bool {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = g.workspaceDir
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	toplevel, err := filepath.Abs(strings.TrimSpace(string(output)))
	if err != nil {
		return false
	}
	// Compare evaluated paths so symlinks do not falsely reject a valid root.
	topEval, err1 := filepath.EvalSymlinks(toplevel)
	wsEval, err2 := filepath.EvalSymlinks(g.workspaceDir)
	if err1 != nil || err2 != nil {
		return filepath.Clean(toplevel) == filepath.Clean(g.workspaceDir)
	}
	return filepath.Clean(topEval) == filepath.Clean(wsEval)
}

// initRepo 初始化 Git 仓库
func (g *GitDetector) initRepo() error {
	cmd := exec.Command("git", "init")
	cmd.Dir = g.workspaceDir
	if err := cmd.Run(); err != nil {
		return err
	}

	// 创建初始 commit
	cmd = exec.Command("git", "add", "-A")
	cmd.Dir = g.workspaceDir
	cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "Initial commit by orchestrator", "--allow-empty")
	cmd.Dir = g.workspaceDir
	cmd.Run()

	return nil
}

// getCurrentCommit 获取当前 commit hash
func (g *GitDetector) getCurrentCommit() string {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = g.workspaceDir
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

// getStatusPorcelain returns a stable git status --porcelain snapshot.
// -z avoids C-style quoting so paths with spaces/non-ASCII fingerprint correctly.
// --untracked-files=all enumerates files under untracked directories so edits
// beneath an existing dirty directory change the fingerprint.
func (g *GitDetector) getStatusPorcelain() string {
	cmd := exec.Command("git", "status", "--porcelain", "-z", "--untracked-files=all")
	cmd.Dir = g.workspaceDir
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return g.filterIgnoredStatus(normalizePorcelainZ(string(output)))
}

// normalizePorcelainZ converts NUL-terminated porcelain into newline records
// with unquoted paths: "XY path\n" (renames keep the destination path only).
func normalizePorcelainZ(raw string) string {
	var b strings.Builder
	i := 0
	for i < len(raw) {
		// Skip empty records produced by trailing NULs.
		if raw[i] == 0 {
			i++
			continue
		}
		end := strings.IndexByte(raw[i:], 0)
		if end < 0 {
			break
		}
		entry := raw[i : i+end]
		i += end + 1
		if len(entry) < 3 {
			continue
		}
		xy := entry[:2]
		path := entry[3:] // after "XY "
		// Rename/copy: PATH1\0PATH2\0 — consume the destination path next.
		if len(xy) == 2 && (xy[0] == 'R' || xy[0] == 'C' || xy[1] == 'R' || xy[1] == 'C') {
			if i < len(raw) {
				end2 := strings.IndexByte(raw[i:], 0)
				if end2 >= 0 {
					path = raw[i : i+end2]
					i += end2 + 1
				}
			}
		}
		if path == "" {
			continue
		}
		b.WriteString(xy)
		b.WriteByte(' ')
		b.WriteString(path)
		b.WriteByte('\n')
	}
	return b.String()
}

func (g *GitDetector) filterIgnoredStatus(status string) string {
	if len(g.ignorePaths) == 0 {
		return status
	}
	var b strings.Builder
	for _, line := range strings.Split(status, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		if len(line) < 4 {
			b.WriteString(line)
			b.WriteByte('\n')
			continue
		}
		path := decodePorcelainPath(strings.TrimSpace(line[3:]))
		if i := strings.Index(path, " -> "); i >= 0 {
			path = path[i+4:]
		}
		if g.isIgnored(path) {
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

func (g *GitDetector) isIgnored(relPath string) bool {
	if len(g.ignorePaths) == 0 {
		return false
	}
	rel := filepath.ToSlash(relPath)
	if g.ignorePaths[rel] {
		return true
	}
	for ignored := range g.ignorePaths {
		if strings.HasPrefix(rel, ignored+"/") {
			return true
		}
	}
	return false
}

// getContentFingerprint hashes working-tree contents of every dirty path so
// edits to already-dirty files count as progress even when porcelain is unchanged.
func (g *GitDetector) getContentFingerprint(status string) string {
	paths := make([]string, 0, len(porcelainFiles(status)))
	for path := range porcelainFiles(status) {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	h := sha256.New()
	for _, path := range paths {
		sum := g.hashWorkspaceFile(path)
		fmt.Fprintf(h, "%s\t%s\n", path, sum)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func (g *GitDetector) hashWorkspaceFile(relPath string) string {
	full := filepath.Join(g.workspaceDir, relPath)
	info, err := os.Lstat(full)
	if err != nil {
		// Missing path (deleted) still contributes a stable sentinel.
		return "missing:" + err.Error()
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(full)
		if err != nil {
			return "missing:" + err.Error()
		}
		sum := sha256.Sum256([]byte("symlink:" + target))
		return hex.EncodeToString(sum[:])
	}
	if info.IsDir() {
		return g.hashDirectory(full)
	}
	if !info.Mode().IsRegular() {
		// Avoid blocking on FIFOs/devices; fingerprint metadata only.
		return fmt.Sprintf("special:%v:%d", info.Mode(), info.Size())
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return "missing:" + err.Error()
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// hashDirectory recursively fingerprints file contents under a dirty directory
// entry (fallback when porcelain still reports a directory path).
func (g *GitDetector) hashDirectory(dir string) string {
	h := sha256.New()
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(g.workspaceDir, path)
		if err != nil {
			return nil
		}
		relSlash := filepath.ToSlash(rel)
		if g.isIgnored(relSlash) {
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				fmt.Fprintf(h, "%s\tmissing:%s\n", relSlash, err.Error())
				return nil
			}
			sum := sha256.Sum256([]byte("symlink:" + target))
			fmt.Fprintf(h, "%s\t%s\n", relSlash, hex.EncodeToString(sum[:]))
			return nil
		}
		if !info.Mode().IsRegular() {
			fmt.Fprintf(h, "%s\tspecial:%v:%d\n", relSlash, info.Mode(), info.Size())
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(h, "%s\tmissing:%s\n", relSlash, err.Error())
			return nil
		}
		sum := sha256.Sum256(data)
		fmt.Fprintf(h, "%s\t%s\n", relSlash, hex.EncodeToString(sum[:]))
		return nil
	})
	return "dir:" + hex.EncodeToString(h.Sum(nil))
}

// checkUntracked 检查是否有未追踪文件
func (g *GitDetector) checkUntracked() bool {
	cmd := exec.Command("git", "ls-files", "--others", "--exclude-standard")
	cmd.Dir = g.workspaceDir
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(strings.TrimSpace(string(output))) > 0
}

// checkModified 检查是否有已修改文件
func (g *GitDetector) checkModified() bool {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = g.workspaceDir
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		// Untracked entries start with "??"; anything else is a tracked change.
		if !strings.HasPrefix(line, "??") {
			return true
		}
	}
	return false
}

// diffStatusFiles returns file paths that differ between two porcelain snapshots.
func (g *GitDetector) diffStatusFiles(before, after string) []string {
	beforeSet := porcelainFiles(before)
	afterSet := porcelainFiles(after)
	var files []string
	for f := range afterSet {
		if !beforeSet[f] {
			files = append(files, f)
		}
	}
	for f := range beforeSet {
		if !afterSet[f] {
			files = append(files, f+" (removed)")
		}
	}
	return files
}

func porcelainFiles(status string) map[string]bool {
	out := make(map[string]bool)
	for _, line := range strings.Split(status, "\n") {
		line = strings.TrimRight(line, "\r")
		if len(line) < 4 {
			continue
		}
		// Porcelain: XY␠path or XY␠orig -> path
		path := decodePorcelainPath(strings.TrimSpace(line[3:]))
		if i := strings.Index(path, " -> "); i >= 0 {
			path = path[i+4:]
		}
		if path != "" {
			out[path] = true
		}
	}
	return out
}

// decodePorcelainPath unquotes C-style quoted porcelain paths (spaces / non-ASCII)
// when status was captured without -z. Paths from -z are returned unchanged.
func decodePorcelainPath(path string) string {
	if len(path) < 2 || path[0] != '"' || path[len(path)-1] != '"' {
		return path
	}
	var b strings.Builder
	b.Grow(len(path) - 2)
	s := path[1 : len(path)-1]
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 >= len(s) {
			b.WriteByte(s[i])
			continue
		}
		i++
		switch s[i] {
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case '"', '\\':
			b.WriteByte(s[i])
		default:
			// Octal escape \NNN
			if s[i] >= '0' && s[i] <= '7' && i+2 < len(s) &&
				s[i+1] >= '0' && s[i+1] <= '7' && s[i+2] >= '0' && s[i+2] <= '7' {
				v := (int(s[i]-'0') << 6) | (int(s[i+1]-'0') << 3) | int(s[i+2]-'0')
				b.WriteByte(byte(v))
				i += 2
			} else {
				b.WriteByte('\\')
				b.WriteByte(s[i])
			}
		}
	}
	return b.String()
}

// CreateCheckpoint 创建检查点（自动 commit）
func (g *GitDetector) CreateCheckpoint(message string) error {
	// Stage all changes
	cmd := exec.Command("git", "add", "-A")
	cmd.Dir = g.workspaceDir
	if err := cmd.Run(); err != nil {
		return err
	}

	// Commit
	cmd = exec.Command("git", "commit", "-m", message, "--allow-empty")
	cmd.Dir = g.workspaceDir
	return cmd.Run()
}
