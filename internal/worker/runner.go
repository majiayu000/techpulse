// Package worker 运行 Claude CLI Worker
package worker

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/anthropic/autonomous-runner/internal/memory"
)

// Result Worker 运行结果
type Result struct {
	Success   bool
	ExitCode  int
	Cost      float64
	Tokens    int
	Duration  time.Duration
	Error     error
	ErrorType ErrorType
	Output    string
}

// ErrorType 错误类型分类
type ErrorType int

const (
	ErrorNone ErrorType = iota
	ErrorTimeout
	ErrorNetwork
	ErrorAuth
	ErrorRateLimit
	ErrorContext
	ErrorUnknown
)

func (e ErrorType) String() string {
	switch e {
	case ErrorNone:
		return "none"
	case ErrorTimeout:
		return "timeout"
	case ErrorNetwork:
		return "network"
	case ErrorAuth:
		return "auth"
	case ErrorRateLimit:
		return "rate_limit"
	case ErrorContext:
		return "context"
	default:
		return "unknown"
	}
}

const (
	// EnvClaudeSkipPermissions is the preferred opt-in for skipping Claude CLI permission prompts.
	EnvClaudeSkipPermissions = "TECHPULSE_CLAUDE_SKIP_PERMISSIONS"
	// EnvAutonomousRunnerSkipPermissions is a legacy alias for the same opt-in.
	EnvAutonomousRunnerSkipPermissions = "AUTONOMOUS_RUNNER_SKIP_PERMISSIONS"
	claudeSkipPermissionsFlag          = "--dangerously-skip-permissions"
)

// Runner Claude CLI 运行器
type Runner struct {
	memory       *memory.Manager
	workspaceDir string
	timeout      time.Duration
	onOutput     func(line string) // 实时输出回调
	// SkipPermissions, when true, appends --dangerously-skip-permissions to the Claude CLI.
	// Default is false. Prefer permission prompts / sandbox for unattended runs.
	// Can also be enabled via TECHPULSE_CLAUDE_SKIP_PERMISSIONS or
	// AUTONOMOUS_RUNNER_SKIP_PERMISSIONS (1/true/yes/on).
	SkipPermissions bool
}

// NewRunner 创建运行器
func NewRunner(mem *memory.Manager, workspaceDir string, timeout time.Duration) *Runner {
	return &Runner{
		memory:       mem,
		workspaceDir: workspaceDir,
		timeout:      timeout,
	}
}

// SetOutputCallback 设置实时输出回调
func (r *Runner) SetOutputCallback(cb func(line string)) {
	r.onOutput = cb
}

// SetSkipPermissions enables or disables the Claude CLI skip-permissions flag.
func (r *Runner) SetSkipPermissions(enabled bool) {
	r.SkipPermissions = enabled
}

// skipPermissionsEnabled reports whether the dangerous skip-permissions flag should be used.
func (r *Runner) skipPermissionsEnabled() bool {
	if r.SkipPermissions {
		return true
	}
	return envTruthy(EnvClaudeSkipPermissions) || envTruthy(EnvAutonomousRunnerSkipPermissions)
}

func envTruthy(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// buildClaudeArgs constructs Claude CLI argv for the given prompt.
// --dangerously-skip-permissions is omitted unless explicitly opted in.
func (r *Runner) buildClaudeArgs(prompt string) []string {
	args := []string{"-p", prompt}
	if r.skipPermissionsEnabled() {
		log.Printf("WARNING: enabling %s via explicit opt-in; this disables Claude CLI filesystem/tool permission prompts and expands local execution risk", claudeSkipPermissionsFlag)
		args = append(args, claudeSkipPermissionsFlag)
	}
	args = append(args, "--output-format", "json")
	return args
}

// Run 运行 Worker
func (r *Runner) Run(ctx context.Context) *Result {
	start := time.Now()
	result := &Result{}

	prompt := r.buildPrompt()

	// 创建带超时的 context
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "claude", r.buildClaudeArgs(prompt)...)
	cmd.Dir = r.workspaceDir
	cmd.Env = os.Environ()

	// 创建管道获取输出
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		result.Error = err
		result.ErrorType = ErrorUnknown
		return result
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		result.Error = err
		result.ErrorType = ErrorUnknown
		return result
	}

	// 启动命令
	if err := cmd.Start(); err != nil {
		result.Error = err
		result.ErrorType = r.classifyError(err.Error())
		return result
	}

	// Collect stdout/stderr into separate builders and join readers before
	// parsing. Concurrent writes to one strings.Builder are unsafe, and
	// StdoutPipe/StderrPipe require drains to finish before relying on Wait.
	var (
		stdoutBuf strings.Builder
		stderrBuf strings.Builder
		wg        sync.WaitGroup
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		r.streamOutput(stdout, &stdoutBuf)
	}()
	go func() {
		defer wg.Done()
		r.streamOutput(stderr, &stderrBuf)
	}()

	err = cmd.Wait()
	wg.Wait()
	result.Duration = time.Since(start)
	output := stdoutBuf.String() + stderrBuf.String()
	result.Output = output

	if ctx.Err() == context.DeadlineExceeded {
		result.Error = fmt.Errorf("worker 超时 (%v)", r.timeout)
		result.ErrorType = ErrorTimeout
		return result
	}

	if err != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
		result.Error = err
		result.ErrorType = r.classifyError(output)
		return result
	}

	result.Success = true
	result.ExitCode = 0

	// 解析成本和 tokens
	r.parseCostAndTokens(output, result)

	return result
}

// streamOutput drains a pipe with an unbounded line reader so large Claude
// JSON records are not truncated by bufio.Scanner's default 64 KiB limit.
func (r *Runner) streamOutput(pipe io.Reader, output *strings.Builder) {
	reader := bufio.NewReader(pipe)
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			output.WriteString(line)
			if r.onOutput != nil {
				r.onOutput(strings.TrimRight(line, "\r\n"))
			}
		}
		if err != nil {
			if err != io.EOF {
				fmt.Fprintf(output, "\n[streamOutput error: %v]\n", err)
			}
			return
		}
	}
}

// classifyError 错误分类
func (r *Runner) classifyError(output string) ErrorType {
	lower := strings.ToLower(output)

	if strings.Contains(lower, "timeout") {
		return ErrorTimeout
	}
	if strings.Contains(lower, "network") || strings.Contains(lower, "connection") {
		return ErrorNetwork
	}
	if strings.Contains(lower, "unauthorized") || strings.Contains(lower, "api key") {
		return ErrorAuth
	}
	if strings.Contains(lower, "rate limit") || strings.Contains(lower, "too many requests") {
		return ErrorRateLimit
	}
	if strings.Contains(lower, "context") || strings.Contains(lower, "token limit") {
		return ErrorContext
	}

	return ErrorUnknown
}

// parseCostAndTokens 解析成本和 tokens
func (r *Runner) parseCostAndTokens(output string, result *Result) {
	// 尝试解析 JSON 格式
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "{") {
			var data map[string]interface{}
			if err := json.Unmarshal([]byte(line), &data); err == nil {
				// Claude CLI 输出 total_cost_usd 字段
				if cost, ok := data["total_cost_usd"].(float64); ok {
					result.Cost = cost
				}
				// 解析 usage 中的 token 统计
				if usage, ok := data["usage"].(map[string]interface{}); ok {
					tokens := 0
					if input, ok := usage["input_tokens"].(float64); ok {
						tokens += int(input)
					}
					if output, ok := usage["output_tokens"].(float64); ok {
						tokens += int(output)
					}
					if cacheRead, ok := usage["cache_read_input_tokens"].(float64); ok {
						tokens += int(cacheRead)
					}
					if cacheCreate, ok := usage["cache_creation_input_tokens"].(float64); ok {
						tokens += int(cacheCreate)
					}
					result.Tokens = tokens
				}
			}
		}
	}

	// 尝试正则解析（备用方案）
	if result.Cost == 0 {
		costPatterns := []string{
			`"total_cost_usd":\s*([\d.]+)`,
			`"cost":\s*([\d.]+)`,
			`Total cost: \$([\d.]+)`,
		}
		for _, pattern := range costPatterns {
			re := regexp.MustCompile(pattern)
			if match := re.FindStringSubmatch(output); len(match) > 1 {
				fmt.Sscanf(match[1], "%f", &result.Cost)
				break
			}
		}
	}

	if result.Tokens == 0 {
		tokenPatterns := []string{
			`"total_tokens":\s*(\d+)`,
			`Total tokens: (\d+)`,
		}
		for _, pattern := range tokenPatterns {
			re := regexp.MustCompile(pattern)
			if match := re.FindStringSubmatch(output); len(match) > 1 {
				fmt.Sscanf(match[1], "%d", &result.Tokens)
				break
			}
		}
	}
}

// buildPrompt 构建 prompt
func (r *Runner) buildPrompt() string {
	ctx, _ := r.memory.ReadContext()
	tasks, _ := r.memory.ReadTasks()
	memDir, _ := r.memory.AbsDir()

	return fmt.Sprintf(`你是一个自主编码 Agent，正在持续执行任务。你的目标是不断改进项目。

## 当前上下文
%s

## 待办任务
%s

## 你的工作流程
1. 阅读上面的上下文，了解当前状态
2. 如果有待办任务（- [ ]），选择一个执行
3. 如果没有待办任务，你需要**自主发现新任务**：
   - 审查代码质量，发现可优化的地方
   - 添加新功能或改进现有功能
   - 重构代码，提高可维护性
   - 添加测试、文档、错误处理
   - 性能优化、用户体验改进
4. 更新记忆文件（路径: %s）：
   - 编辑 TASKS.md：将完成的任务标记为 [x]，添加新发现的任务
   - 编辑 CONTEXT.md：更新"最近完成的工作"和"下一步建议"
   - 在 DONE.md 追加一行记录：| %s | 任务描述 | 备注 |

## 重要规则
- 每次只完成 1-2 个任务，不要贪多
- **永远保持 TASKS.md 中有未完成的任务**（自己发现新任务）
- 如果任务太大，拆分成子任务添加到 TASKS.md
- 完成后必须更新 .md 文件，这是你和下一次运行沟通的方式
- 持续改进，没有"完成"的概念，总有可以优化的地方

## 开始工作
查看任务列表，如果为空则自主发现新任务，然后执行。`,
		ctx, tasks, memDir, time.Now().Format("2006-01-02 15:04"))
}
