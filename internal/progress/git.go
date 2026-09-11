package progress

import (
	"fmt"
	"os/exec"
	"strings"
)

// GitDetector 基于 Git 检测代码变化
type GitDetector struct {
	workspaceDir   string
	lastCommit     string
	baselineStatus string // porcelain snapshot taken at Reset
	hasUntracked   bool
	hasModified    bool
	changedFiles   []string
	statusChanged  bool
	commitChanged  bool
}

// NewGitDetector 创建 Git 检测器
func NewGitDetector(workspaceDir string) (*GitDetector, error) {
	g := &GitDetector{workspaceDir: workspaceDir}

	// 检查是否是 Git 仓库
	if !g.isGitRepo() {
		// 初始化 Git 仓库
		if err := g.initRepo(); err != nil {
			return nil, fmt.Errorf("初始化 Git 仓库失败: %w", err)
		}
	}

	// 获取当前 commit 与初始状态快照
	g.lastCommit = g.getCurrentCommit()
	g.baselineStatus = g.getStatusPorcelain()

	return g, nil
}

func (g *GitDetector) Name() string {
	return "git"
}

func (g *GitDetector) Detect() (bool, error) {
	currentStatus := g.getStatusPorcelain()
	currentCommit := g.getCurrentCommit()

	g.commitChanged = currentCommit != g.lastCommit && currentCommit != ""
	g.statusChanged = currentStatus != g.baselineStatus

	g.hasUntracked = g.checkUntracked()
	g.hasModified = g.checkModified()
	g.changedFiles = g.diffStatusFiles(g.baselineStatus, currentStatus)

	hasProgress := g.statusChanged || g.commitChanged

	if g.commitChanged {
		g.lastCommit = currentCommit
	}

	return hasProgress, nil
}

func (g *GitDetector) Reset() error {
	g.lastCommit = g.getCurrentCommit()
	g.baselineStatus = g.getStatusPorcelain()
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
	if g.statusChanged {
		if g.hasUntracked {
			parts = append(parts, "新文件")
		}
		if g.hasModified {
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

// isGitRepo 检查目录是否是 Git 仓库
func (g *GitDetector) isGitRepo() bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = g.workspaceDir
	return cmd.Run() == nil
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

// getStatusPorcelain 返回稳定的 git status --porcelain 快照
func (g *GitDetector) getStatusPorcelain() string {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = g.workspaceDir
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return string(output)
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
		path := strings.TrimSpace(line[3:])
		if i := strings.Index(path, " -> "); i >= 0 {
			path = path[i+4:]
		}
		if path != "" {
			out[path] = true
		}
	}
	return out
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
