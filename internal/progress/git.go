package progress

import (
	"fmt"
	"os/exec"
	"strings"
)

// GitDetector 基于 Git 检测代码变化
type GitDetector struct {
	workspaceDir  string
	lastCommit    string
	hasUntracked  bool
	hasModified   bool
	changedFiles  []string
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

	// 获取当前 commit
	g.lastCommit = g.getCurrentCommit()

	return g, nil
}

func (g *GitDetector) Name() string {
	return "git"
}

func (g *GitDetector) Detect() (bool, error) {
	// 检查未追踪文件
	g.hasUntracked = g.checkUntracked()

	// 检查已修改文件
	g.hasModified = g.checkModified()

	// 检查新 commit
	currentCommit := g.getCurrentCommit()
	newCommit := currentCommit != g.lastCommit && currentCommit != ""

	// 获取变更文件列表
	g.changedFiles = g.getChangedFiles()

	hasProgress := g.hasUntracked || g.hasModified || newCommit

	if newCommit {
		g.lastCommit = currentCommit
	}

	return hasProgress, nil
}

func (g *GitDetector) Reset() error {
	g.lastCommit = g.getCurrentCommit()
	g.hasUntracked = false
	g.hasModified = false
	g.changedFiles = nil
	return nil
}

func (g *GitDetector) Details() string {
	var parts []string
	if g.hasUntracked {
		parts = append(parts, "新文件")
	}
	if g.hasModified {
		parts = append(parts, "已修改")
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
	return len(strings.TrimSpace(string(output))) > 0
}

// getChangedFiles 获取变更文件列表
func (g *GitDetector) getChangedFiles() []string {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = g.workspaceDir
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	var files []string
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) > 3 {
			files = append(files, line[3:])
		}
	}
	return files
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
