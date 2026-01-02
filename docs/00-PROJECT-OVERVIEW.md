# TechPulse - AI/科技信息收集器

## 项目概览

TechPulse 是一个自主运行的 AI/科技信息收集器，能够自动从 Hacker News、RSS 等多个数据源收集科技资讯，并通过 AI 驱动的过滤和评分系统提供高质量的信息摘要。

## 快速开始

### 编译

```bash
go build -o techpulse ./cmd/techpulse
```

### 运行单次收集

```bash
./techpulse
```

### 持续运行模式

```bash
./techpulse -continuous -interval 30m
```

### 清理旧数据

```bash
./techpulse -cleanup -retention 30
```

## 命令行参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-continuous` | false | 持续运行模式 |
| `-interval` | 30m | 收集间隔 |
| `-storage` | .techpulse | 存储目录 |
| `-limit` | 50 | 每个数据源的最大收集数量 |
| `-threshold` | 6.0 | 重要性阈值 (0-10) |
| `-cleanup` | false | 清理旧归档 |
| `-retention` | 30 | 归档保留天数 |

## 项目结构

```
auto-run-agent/
├── cmd/
│   └── techpulse/              # TechPulse CLI 入口
│       └── main.go
├── docs/
│   ├── 00-PROJECT-OVERVIEW.md  # 本文档
│   ├── 01-PRD.md               # 产品需求文档
│   ├── 02-TECHNICAL-SPEC.md    # 技术规格文档
│   └── 03-ARCHITECTURE.md      # 模块架构设计
├── internal/
│   ├── collector/              # 数据收集模块
│   │   ├── collector.go        # 接口定义
│   │   ├── registry.go         # 收集器注册表
│   │   ├── hackernews/         # Hacker News 收集器
│   │   └── rss/                # RSS 收集器
│   ├── filter/                 # 过滤模块
│   │   ├── filter.go           # 接口定义
│   │   ├── keyword.go          # 关键词过滤
│   │   ├── dedup.go            # 去重过滤
│   │   └── pipeline.go         # 过滤管道
│   ├── summarizer/             # 摘要模块
│   │   ├── summarizer.go       # 接口定义
│   │   └── basic.go            # 基础实现
│   ├── storage/                # 存储模块
│   │   ├── storage.go          # 接口定义
│   │   └── markdown.go         # Markdown 存储
│   ├── techpulse/              # 主协调器
│   │   ├── techpulse.go        # 核心逻辑
│   │   └── integration.go      # Memory 集成
│   ├── config/                 # 配置管理 (现有)
│   ├── memory/                 # 记忆管理 (现有)
│   ├── worker/                 # 工作器 (现有)
│   └── progress/               # 进度检测 (现有)
└── memory/
    ├── TASKS.md                # 任务列表
    ├── CONTEXT.md              # 项目上下文
    └── DONE.md                 # 完成历史
```

## 数据源

### 已实现

| 数据源 | 类型 | 状态 | 说明 |
|--------|------|------|------|
| Hacker News Top | API | ✅ 可用 | 热门故事 |
| Hacker News New | API | ✅ 可用 | 最新故事 |
| Hacker News Best | API | ✅ 可用 | 最佳故事 |
| TechCrunch | RSS | ✅ 可用 | 科技新闻 |
| Ars Technica | RSS | ✅ 可用 | 科技深度报道 |
| The Verge | RSS | ✅ 可用 | 科技资讯 |
| Wired | RSS | ✅ 可用 | 科技与文化 |
| MIT Tech Review | RSS | ✅ 可用 | 技术评论 |
| Hacker Noon | RSS | ✅ 可用 | 开发者文章 |

### 计划中

| 数据源 | 类型 | 优先级 | 说明 |
|--------|------|--------|------|
| Reddit | API | 高 | r/MachineLearning, r/programming |
| Twitter/X | 第三方 API | 低 | 成本高，v1.2 考虑 |

## 输出文件

### DIGEST.md

每日摘要报告，包含：
- 收集统计信息
- 热门趋势
- Top 10 重要文章

### archive/YYYY-MM-DD.md

每日归档，按来源分组的完整文章列表。

## 关键词过滤

### 默认包含

```
AI, artificial intelligence, machine learning, ML, LLM, GPT, Claude, ChatGPT,
Gemini, neural network, deep learning, transformer, Rust, Go, TypeScript,
Python, open source, developer, programming, startup, tech, software
```

### 默认排除

```
crypto, NFT, blockchain, bitcoin, gambling, casino, porn
```

## 与 autonomous-runner 集成

TechPulse 设计为与现有的 autonomous-runner 框架无缝集成：

1. **Memory 集成**: 自动更新 CONTEXT.md 和 DONE.md
2. **Progress 检测**: 可通过文件哈希检测进度变化
3. **持续运行**: 支持作为后台任务持续执行

### 示例：作为自主任务运行

```go
// 创建 TechPulse 实例
tp := techpulse.New(techpulse.DefaultConfig())

// 创建 Memory 集成
memManager, _ := memory.NewManager("./memory")
memIntegration := techpulse.NewMemoryIntegration(memManager)

// 创建自主运行器
runner := techpulse.NewAutonomousRunner(tp, memIntegration, 30*time.Minute)

// 运行
ctx := context.Background()
runner.RunLoop(ctx)
```

## 开发路线图

### Phase 1: MVP ✅

- [x] Hacker News 收集器
- [x] RSS 收集器
- [x] 关键词过滤
- [x] 去重过滤
- [x] 基础重要性评分
- [x] Markdown 存储
- [x] CLI 入口
- [x] Memory 集成

### Phase 2: AI 增强

- [ ] Claude API 集成
- [ ] AI 驱动的摘要生成
- [ ] 智能重要性评分
- [ ] 趋势分析

### Phase 3: 扩展数据源

- [ ] Reddit 收集器
- [ ] 每日邮件报告
- [ ] Web UI

## 文档索引

| 文档 | 说明 |
|------|------|
| [01-PRD.md](01-PRD.md) | 产品需求文档，包含用户故事和验收标准 |
| [02-TECHNICAL-SPEC.md](02-TECHNICAL-SPEC.md) | 技术规格，包含 C4 架构图和 API 设计 |
| [03-ARCHITECTURE.md](03-ARCHITECTURE.md) | 模块架构，严格遵循 200 行文件限制 |

## 许可证

MIT License

---

*由 Claude AI 使用 autonomous-runner 框架生成*
