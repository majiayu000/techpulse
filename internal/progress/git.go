package progress

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// GitDetector 基于 Git 检测代码变化
type GitDetector struct {
	workspaceDir string
	lastCommit   string
	hasUntracked bool
	hasModified  bool
	changedFiles []string
}

// NewGitDetector 创建 Git 检测器。git 缺失或不可执行时返回错误（fail closed），
// 不再静默把失败当作“非仓库”继续初始化。
func NewGitDetector(workspaceDir string) (*GitDetector, error) {
	g := &GitDetector{workspaceDir: workspaceDir}

	// 检查是否是 Git 仓库
	isRepo, err := g.isGitRepo()
	if err != nil {
		return nil, fmt.Errorf("检查 Git 仓库失败: %w", err)
	}
	if !isRepo {
		// 初始化 Git 仓库
		if err := g.initRepo(); err != nil {
			return nil, fmt.Errorf("初始化 Git 仓库失败: %w", err)
		}
	}

	// 获取当前 commit
	commit, err := g.getCurrentCommit()
	if err != nil {
		return nil, fmt.Errorf("获取当前 commit 失败: %w", err)
	}
	g.lastCommit = commit

	return g, nil
}

func (g *GitDetector) Name() string { return "git" }

func (g *GitDetector) Detect() (bool, error) {
	// 检查未追踪文件
	hasUntracked, err := g.checkUntracked()
	if err != nil {
		return false, fmt.Errorf("git 检查未追踪文件失败: %w", err)
	}
	g.hasUntracked = hasUntracked

	// 检查已修改文件
	hasModified, err := g.checkModified()
	if err != nil {
		return false, fmt.Errorf("git 检查已修改文件失败: %w", err)
	}
	g.hasModified = hasModified

	// 检查新 commit
	currentCommit, err := g.getCurrentCommit()
	if err != nil {
		return false, fmt.Errorf("git 获取当前 commit 失败: %w", err)
	}
	newCommit := currentCommit != g.lastCommit && currentCommit != ""

	// 获取变更文件列表
	changedFiles, err := g.getChangedFiles()
	if err != nil {
		return false, fmt.Errorf("git 获取变更文件列表失败: %w", err)
	}
	g.changedFiles = changedFiles

	hasProgress := g.hasUntracked || g.hasModified || newCommit

	if newCommit {
		g.lastCommit = currentCommit
	}

	return hasProgress, nil
}

func (g *GitDetector) Reset() error {
	commit, err := g.getCurrentCommit()
	if err != nil {
		return fmt.Errorf("git 重置失败: %w", err)
	}
	g.lastCommit = commit
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
		// 复制后再截断，避免 append 把“...等 N 个文件”写进共享底层数组，
		// 污染 g.changedFiles 里保存的真实文件名
		files := make([]string, 0, 6)
		limit := len(g.changedFiles)
		if limit > 5 {
			limit = 5
		}
		files = append(files, g.changedFiles[:limit]...)
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

// gitOutput 在 workspaceDir 中运行 git 命令并返回 stdout（已去除首尾空白）。
// absent=true 表示合法的“无数据”情形——目录不是 git 仓库，或仓库尚无提交——
// 它不是错误；其余失败（git 缺失、权限、目录不可用等）作为 error 返回，不再静默吞掉。
func (g *GitDetector) gitOutput(args ...string) (stdout string, absent bool, absentErr error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = g.workspaceDir
	output, err := cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(output)), false, nil
	}
	if isGitAbsence(err) {
		return "", true, nil
	}
	return "", false, fmt.Errorf("git %s 失败: %w", strings.Join(args, " "), err)
}

// isGitAbsence 判断 git 的非零退出是否属于合法的“无数据”而非真实故障：
// 目录不是 git 仓库，或 HEAD 尚不存在（空仓库）。git 缺失等其它错误返回 false。
func isGitAbsence(err error) bool {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return false
	}
	msg := string(exitErr.Stderr)
	return strings.Contains(msg, "not a git repository") ||
		strings.Contains(msg, "unknown revision")
}

// isGitRepo 检查目录是否是 Git 仓库
func (g *GitDetector) isGitRepo() (bool, error) {
	_, absent, err := g.gitOutput("rev-parse", "--git-dir")
	if err != nil {
		return false, err
	}
	return !absent, nil
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

// getCurrentCommit 获取当前 commit hash；空仓库返回 ""（属合法“无数据”，不是错误）
func (g *GitDetector) getCurrentCommit() (string, error) {
	out, _, err := g.gitOutput("rev-parse", "HEAD")
	return out, err
}

// checkUntracked 检查是否有未追踪文件
func (g *GitDetector) checkUntracked() (bool, error) {
	out, _, err := g.gitOutput("ls-files", "--others", "--exclude-standard")
	if err != nil {
		return false, err
	}
	return len(out) > 0, nil
}

// checkModified 检查是否有已修改文件
func (g *GitDetector) checkModified() (bool, error) {
	out, _, err := g.gitOutput("status", "--porcelain")
	if err != nil {
		return false, err
	}
	return len(out) > 0, nil
}

// getChangedFiles 获取变更文件列表
func (g *GitDetector) getChangedFiles() ([]string, error) {
	out, _, err := g.gitOutput("status", "--porcelain")
	if err != nil {
		return nil, err
	}
	var files []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if len(line) > 3 {
			files = append(files, line[3:])
		}
	}
	return files, nil
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
