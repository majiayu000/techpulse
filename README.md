# Claude Code Autonomous Runner

让 Claude Code 24小时自主运行的最稳定方案。

## 核心设计

```
┌────────────────────────────────────────────────────────────┐
│                    Orchestrator                             │
│              (轻量级调度器，永远不会崩溃)                     │
│    • 健康检查   • 成本监控   • 智能终止                      │
├────────────────────────────────────────────────────────────┤
│                    Memory Layer                             │
│              (Markdown 文件，跨会话共享)                      │
│    TASKS.md → CONTEXT.md → DONE.md                          │
├────────────────────────────────────────────────────────────┤
│                    Worker (Claude CLI)                      │
│              (单次任务后退出)                                │
│    Claude Code (permission prompts by default)              │
└────────────────────────────────────────────────────────────┘
```

## 快速开始

### 1. 确保已登录 Claude Code

```bash
# 如果还没登录，先运行一次
claude
```

### 2. 准备项目目录

项目目录需要包含 `memory/`、`workspace/`、`logs/`，Orchestrator 会自动创建：

```
project/
├── memory/
├── workspace/
└── logs/
```

### 3. 编辑任务

```bash
vim project/memory/TASKS.md
```

添加你的任务：
```markdown
## 高优先级
- [ ] 为 src/auth.ts 添加单元测试
- [ ] 修复登录页面的类型错误

## 中优先级
- [ ] 重构 UserService 类
```

### 4. 编辑上下文

```bash
vim project/memory/CONTEXT.md
```

描述你的项目，帮助 Claude 理解背景。

### 5. 启动

```bash
chmod +x run.sh
./run.sh 100 50 8 /path/to/project
```

## 命令

```bash
./run.sh                    # 默认: 100 次, $50, 8 小时, 项目目录为当前目录
./run.sh 10 5 2 /path/to/project

# 也可直接运行二进制
./orchestrator --dir /path/to/project --max-iterations 10 --max-cost 5 --max-duration 2
```

## 配置

在项目目录放置 `config.yaml`（可选）：

```yaml
# 安全限制
max_iterations: 100         # 最大迭代次数
max_cost_usd: 50.0          # 最大成本 (美元)
max_duration: 8h            # 最大运行时长 (Go duration)
consecutive_no_progress: 3  # 连续无进展后停止
stop_when_empty: true       # 任务为空时停止

# 执行配置
cooldown_duration: 10s       # 迭代间隔
worker_timeout: 30m          # 单次 worker 超时

# 进展检测
use_git_detection: true      # 基于 workspace 的 Git 变化检测
```

## 文件结构

```
autonomous-runner/
├── cmd/orchestrator/    # 主入口
├── internal/            # 核心逻辑
├── run.sh               # 启动脚本
└── (project dir)        # 运行时项目目录
    ├── memory/          # 外部记忆（Claude 读写）
    ├── workspace/       # 工作目录（代码在这里）
    └── logs/            # 运行日志
```

## 工作原理

### 为什么这样设计？

| 设计决策 | 解决的问题 |
|---------|-----------|
| Worker 短命 | Context window 永不耗尽 |
| 进程短命 | Context window 永不耗尽 |
| Markdown 记忆 | 人可读、可编辑、可版本控制 |
| 多重终止条件 | 防止无限循环和成本失控 |
| 无进展检测 | 防止重复劳动 |

### 执行流程

```
Orchestrator 启动
     ↓
┌→ 检查是否继续 (成本/时间/迭代/任务)
│    ↓ (继续)
│  启动 Worker (Claude CLI)
│    ↓
│  Worker 读取 TASKS.md + CONTEXT.md
│    ↓
│  Worker 执行任务
│    ↓
│  Worker 更新 .md 文件
│    ↓
│  Worker 退出
│    ↓
│  Orchestrator 检测进展
│    ↓
│  冷却等待
│    ↓
└──────────────────┘
     ↓ (停止)
打印摘要，退出
```

## 高级用法

### 把代码放入 workspace

```bash
# 方式1: 克隆到 workspace
git clone https://github.com/your/repo /path/to/project/workspace

# 方式2: 软链接已有项目
ln -s /path/to/your/project /path/to/project/workspace
```

### 后台运行

```bash
# 使用 tmux
tmux new -s claude
./run.sh 100 50 8 /path/to/project
# Ctrl+B, D 分离

# 使用 nohup
nohup ./run.sh 100 50 8 /path/to/project > /dev/null 2>&1 &
```

### 监控日志

```bash
# 实时查看日志
tail -f logs/orchestrator_*.log

# 查看状态
tail -f /path/to/project/logs/orchestrator_*.log
```

## 安全说明

1. **成本限制**：`max_cost_usd` 硬限制 API 支出
2. **时间限制**：`max_duration` 防止无限运行
3. **无进展检测**：连续无变化自动停止
4. **优雅退出**：Ctrl+C 会完成当前任务后停止
5. **权限模式（默认安全）**：Worker **默认不**传 `--dangerously-skip-permissions`，保留 Claude CLI 权限提示。仅在明确需要无人值守且充分信任工作区时，通过显式 opt-in 开启：
   - 环境变量：`TECHPULSE_CLAUDE_SKIP_PERMISSIONS=true`（或别名 `AUTONOMOUS_RUNNER_SKIP_PERMISSIONS=true`）
   - 代码：`Runner.SkipPermissions = true` / `SetSkipPermissions(true)`
   - 开启后会禁用文件系统/工具权限提示，扩大本地执行风险；详见 [SECURITY.md](SECURITY.md)

## 常见问题

### Q: 如何查看 Claude 做了什么？

```bash
# 查看完成历史
cat memory/DONE.md

# 查看日志
tail -100 logs/orchestrator_*.log
```

### Q: 如何干预正在运行的任务？

1. 直接编辑 `memory/TASKS.md` 添加紧急任务
2. 下一次迭代 Claude 会看到新任务

### Q: 成本如何计算？

目前通过 Claude Code 的 JSON 输出解析成本。如果解析失败，建议监控 Anthropic 控制台。

### Q: 可以用 Git 检测进展吗？

可以，默认开启 `use_git_detection: true`，会检测 `workspace/` 内的变更。

## License

MIT
