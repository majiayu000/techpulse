# TechPulse - 模块架构设计

**作者**: Claude AI
**状态**: Proposed
**创建日期**: 2026-01-01

---

## 1. 架构原则

### 核心约束

- **200 行限制**: 每个文件不超过 200 行代码
- **单一职责**: 每个模块只做一件事
- **接口优先**: 先定义接口，后实现
- **依赖注入**: 松耦合设计
- **真实测试**: 不使用 mock，测试真实行为

---

## 2. 目录结构

```
internal/
├── collector/                    # 数据收集模块
│   ├── collector.go             # Collector 接口定义 (~80 lines)
│   ├── registry.go              # 收集器注册表 (~50 lines)
│   ├── options.go               # 收集选项 (~40 lines)
│   │
│   ├── hackernews/              # HN 收集器
│   │   ├── collector.go         # 主收集逻辑 (~150 lines)
│   │   ├── types.go             # HN 数据类型 (~40 lines)
│   │   └── client.go            # HTTP 客户端 (~80 lines)
│   │
│   ├── reddit/                  # Reddit 收集器
│   │   ├── collector.go         # 主收集逻辑 (~150 lines)
│   │   ├── types.go             # Reddit 数据类型 (~60 lines)
│   │   ├── auth.go              # OAuth 认证 (~100 lines)
│   │   └── client.go            # API 客户端 (~120 lines)
│   │
│   ├── rss/                     # RSS 收集器
│   │   ├── collector.go         # 主收集逻辑 (~120 lines)
│   │   ├── parser.go            # RSS 解析 (~100 lines)
│   │   └── sources.go           # 预定义源 (~50 lines)
│   │
│   └── twitter/                 # Twitter 收集器 (可选)
│       ├── collector.go         # 主收集逻辑 (~150 lines)
│       └── client.go            # API 客户端 (~100 lines)
│
├── filter/                       # 过滤模块
│   ├── filter.go                # Filter 接口 (~50 lines)
│   ├── keyword.go               # 关键词过滤 (~100 lines)
│   ├── dedup.go                 # 去重过滤 (~80 lines)
│   ├── importance.go            # 重要性基础评分 (~80 lines)
│   └── pipeline.go              # 过滤管道 (~60 lines)
│
├── summarizer/                   # 摘要模块
│   ├── summarizer.go            # Summarizer 接口 (~60 lines)
│   ├── claude.go                # Claude 实现 (~150 lines)
│   ├── scoring.go               # 评分逻辑 (~80 lines)
│   └── report.go                # 报告生成 (~120 lines)
│
├── storage/                      # 存储模块
│   ├── storage.go               # Storage 接口 (~50 lines)
│   ├── markdown.go              # Markdown 存储 (~150 lines)
│   ├── archive.go               # 归档管理 (~80 lines)
│   └── sqlite.go                # SQLite 存储 (可选, ~150 lines)
│
├── integration/                  # 集成模块
│   ├── memory.go                # Memory 模块集成 (~100 lines)
│   ├── worker.go                # Worker 模块集成 (~80 lines)
│   └── progress.go              # Progress 模块集成 (~80 lines)
│
└── scheduler/                    # 调度模块
    ├── scheduler.go             # 调度器接口 (~60 lines)
    ├── cron.go                  # Cron 调度 (~100 lines)
    └── runner.go                # 运行器 (~120 lines)
```

---

## 3. 模块详细设计

### 3.1 Collector 模块

#### 核心接口 (`collector/collector.go`)

```go
package collector

import (
    "context"
    "time"
)

// Article 收集到的文章
type Article struct {
    ID          string
    Source      string
    SourceID    string
    Title       string
    URL         string
    Content     string
    Author      string
    Score       int
    Comments    int
    Tags        []string
    Metadata    map[string]string
    PublishedAt time.Time
    CollectedAt time.Time
}

// Collector 收集器接口
type Collector interface {
    Name() string
    Collect(ctx context.Context, opts Options) ([]Article, error)
    Validate() error
}

// Result 收集结果
type Result struct {
    Source    string
    Articles  []Article
    Error     error
    Duration  time.Duration
    Timestamp time.Time
}
```

#### 注册表 (`collector/registry.go`)

```go
package collector

import "sync"

// Registry 收集器注册表
type Registry struct {
    mu         sync.RWMutex
    collectors map[string]Collector
}

func NewRegistry() *Registry {
    return &Registry{
        collectors: make(map[string]Collector),
    }
}

func (r *Registry) Register(c Collector) error {
    if err := c.Validate(); err != nil {
        return err
    }
    r.mu.Lock()
    defer r.mu.Unlock()
    r.collectors[c.Name()] = c
    return nil
}

func (r *Registry) Get(name string) (Collector, bool) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    c, ok := r.collectors[name]
    return c, ok
}

func (r *Registry) All() []Collector {
    r.mu.RLock()
    defer r.mu.RUnlock()
    result := make([]Collector, 0, len(r.collectors))
    for _, c := range r.collectors {
        result = append(result, c)
    }
    return result
}

func (r *Registry) CollectAll(ctx context.Context, opts Options) []Result {
    collectors := r.All()
    results := make([]Result, len(collectors))

    // 并发收集
    var wg sync.WaitGroup
    for i, c := range collectors {
        wg.Add(1)
        go func(idx int, col Collector) {
            defer wg.Done()
            start := time.Now()
            articles, err := col.Collect(ctx, opts)
            results[idx] = Result{
                Source:    col.Name(),
                Articles:  articles,
                Error:     err,
                Duration:  time.Since(start),
                Timestamp: time.Now(),
            }
        }(i, c)
    }
    wg.Wait()

    return results
}
```

#### HN 收集器文件拆分

```
hackernews/
├── collector.go    # 主逻辑: New(), Name(), Validate(), Collect()
├── types.go        # HN API 响应类型
└── client.go       # HTTP 请求封装
```

### 3.2 Filter 模块

#### 过滤接口 (`filter/filter.go`)

```go
package filter

import "github.com/anthropic/autonomous-runner/internal/collector"

// FilteredArticle 过滤后的文章
type FilteredArticle struct {
    collector.Article
    MatchedKeywords []string
    IsDuplicate     bool
    DuplicateOf     string
    BaseScore       float64
}

// Filter 过滤器接口
type Filter interface {
    Name() string
    Apply(articles []collector.Article) []FilteredArticle
}

// FilterFunc 函数式过滤器
type FilterFunc func([]collector.Article) []FilteredArticle

func (f FilterFunc) Name() string { return "func" }
func (f FilterFunc) Apply(a []collector.Article) []FilteredArticle { return f(a) }
```

#### 过滤管道 (`filter/pipeline.go`)

```go
package filter

// Pipeline 过滤管道
type Pipeline struct {
    filters []Filter
}

func NewPipeline(filters ...Filter) *Pipeline {
    return &Pipeline{filters: filters}
}

func (p *Pipeline) Add(f Filter) *Pipeline {
    p.filters = append(p.filters, f)
    return p
}

func (p *Pipeline) Process(articles []collector.Article) []FilteredArticle {
    if len(p.filters) == 0 {
        return toFiltered(articles)
    }

    result := p.filters[0].Apply(articles)

    for i := 1; i < len(p.filters); i++ {
        // 转换回 Article 以应用下一个过滤器
        result = p.filters[i].Apply(toArticles(result))
    }

    return result
}

func toFiltered(articles []collector.Article) []FilteredArticle {
    result := make([]FilteredArticle, len(articles))
    for i, a := range articles {
        result[i] = FilteredArticle{Article: a}
    }
    return result
}

func toArticles(filtered []FilteredArticle) []collector.Article {
    result := make([]collector.Article, len(filtered))
    for i, f := range filtered {
        result[i] = f.Article
    }
    return result
}
```

### 3.3 Summarizer 模块

#### 接口定义 (`summarizer/summarizer.go`)

```go
package summarizer

import (
    "context"

    "github.com/anthropic/autonomous-runner/internal/filter"
)

// EnrichedArticle AI 增强后的文章
type EnrichedArticle struct {
    filter.FilteredArticle
    Importance float64
    Summary    string
    AITags     []string
    Reasoning  string
}

// Summarizer 摘要接口
type Summarizer interface {
    Enrich(ctx context.Context, articles []filter.FilteredArticle) ([]EnrichedArticle, error)
    GenerateReport(ctx context.Context, articles []EnrichedArticle) (Report, error)
}

// Report 每日报告
type Report struct {
    Title      string
    Date       string
    TopStories []EnrichedArticle
    Trends     []Trend
    Stats      Stats
    Content    string // Markdown 内容
}

// Trend 趋势
type Trend struct {
    Topic    string
    Count    int
    Momentum float64 // 增长趋势
}

// Stats 统计
type Stats struct {
    TotalArticles int
    BySource      map[string]int
    AvgImportance float64
    TopKeywords   []string
}
```

### 3.4 Storage 模块

#### 存储接口 (`storage/storage.go`)

```go
package storage

import (
    "time"

    "github.com/anthropic/autonomous-runner/internal/summarizer"
)

// Storage 存储接口
type Storage interface {
    Save(articles []summarizer.EnrichedArticle) error
    SaveReport(report summarizer.Report) error
    Load(since time.Time) ([]summarizer.EnrichedArticle, error)
    Archive(before time.Time) error
}

// Config 存储配置
type Config struct {
    BaseDir       string
    DigestFile    string
    ArchiveDir    string
    RetentionDays int
}

func DefaultConfig(baseDir string) Config {
    return Config{
        BaseDir:       baseDir,
        DigestFile:    "DIGEST.md",
        ArchiveDir:    "archive",
        RetentionDays: 30,
    }
}
```

### 3.5 Integration 模块

#### Memory 集成 (`integration/memory.go`)

```go
package integration

import (
    "fmt"
    "strings"
    "time"

    "github.com/anthropic/autonomous-runner/internal/memory"
    "github.com/anthropic/autonomous-runner/internal/summarizer"
)

// MemoryBridge Memory 模块桥接
type MemoryBridge struct {
    manager *memory.Manager
}

func NewMemoryBridge(m *memory.Manager) *MemoryBridge {
    return &MemoryBridge{manager: m}
}

// UpdateContext 更新 CONTEXT.md
func (b *MemoryBridge) UpdateContext(articles []summarizer.EnrichedArticle) error {
    content := b.formatContextUpdate(articles)
    return b.manager.UpdateSection("collection", content)
}

// AddTask 添加任务到 TASKS.md
func (b *MemoryBridge) AddTask(description string) error {
    task := fmt.Sprintf("- [ ] %s", description)
    return b.manager.AddTask(task)
}

// MarkDone 记录完成到 DONE.md
func (b *MemoryBridge) MarkDone(source string, count int) error {
    entry := fmt.Sprintf("| %s | Collected %d from %s |",
        time.Now().Format("2006-01-02 15:04"), count, source)
    return b.manager.AddDone(entry)
}

func (b *MemoryBridge) formatContextUpdate(articles []summarizer.EnrichedArticle) string {
    var sb strings.Builder
    sb.WriteString("## Recent Collection\n\n")
    sb.WriteString(fmt.Sprintf("- Time: %s\n", time.Now().Format(time.RFC3339)))
    sb.WriteString(fmt.Sprintf("- Articles: %d\n", len(articles)))

    if len(articles) > 0 {
        sb.WriteString("\n### Highlights\n")
        for i, a := range articles {
            if i >= 3 {
                break
            }
            sb.WriteString(fmt.Sprintf("- [%.1f] %s\n", a.Importance, a.Title))
        }
    }

    return sb.String()
}
```

### 3.6 Scheduler 模块

#### 调度器 (`scheduler/scheduler.go`)

```go
package scheduler

import (
    "context"
    "time"
)

// Job 调度任务
type Job struct {
    Name     string
    Interval time.Duration
    Execute  func(ctx context.Context) error
    LastRun  time.Time
    NextRun  time.Time
}

// Scheduler 调度器接口
type Scheduler interface {
    Schedule(job Job) error
    Start(ctx context.Context) error
    Stop() error
    Status() []JobStatus
}

// JobStatus 任务状态
type JobStatus struct {
    Name      string
    LastRun   time.Time
    NextRun   time.Time
    LastError error
    RunCount  int
}
```

---

## 4. 依赖关系

```
┌─────────────────────────────────────────────────────────────────┐
│                        Dependency Graph                          │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  scheduler ─────────────────────────────────────────────────┐   │
│      │                                                       │   │
│      ▼                                                       │   │
│  integration ──────────────────────────────────────┐         │   │
│      │                                              │         │   │
│      ├──────────▶ memory (existing)                 │         │   │
│      ├──────────▶ worker (existing)                 │         │   │
│      └──────────▶ progress (existing)               │         │   │
│                                                     │         │   │
│  storage ◀────────────────────────────────────────┐│         │   │
│      ▲                                            ││         │   │
│      │                                            ││         │   │
│  summarizer ──────────────────────────────────────┤│         │   │
│      ▲                                            ││         │   │
│      │                                            ││         │   │
│  filter ──────────────────────────────────────────┤│         │   │
│      ▲                                            ││         │   │
│      │                                            ││         │   │
│  collector ───────────────────────────────────────┘│         │   │
│      │                                             │         │   │
│      ├── hackernews/                               │         │   │
│      ├── reddit/                                   │         │   │
│      ├── rss/                                      │         │   │
│      └── twitter/ (optional)                       │         │   │
│                                                    │         │   │
│  config (existing) ◀───────────────────────────────┴─────────┘   │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘

依赖方向: 上层 → 下层
无循环依赖
```

---

## 5. 配置扩展

```yaml
# config.yaml 扩展

# 现有配置...
limits:
  max_iterations: 100
  max_cost: 50
  max_duration_hours: 8

# 新增 TechPulse 配置
techpulse:
  # 收集器配置
  collectors:
    hackernews:
      enabled: true
      categories: ["top", "new", "best"]
      limit_per_category: 30

    reddit:
      enabled: true
      subreddits:
        - "MachineLearning"
        - "artificial"
        - "programming"
        - "technology"
      limit_per_subreddit: 20
      # OAuth 配置
      client_id: "${REDDIT_CLIENT_ID}"
      client_secret: "${REDDIT_CLIENT_SECRET}"

    rss:
      enabled: true
      sources:
        - name: "TechCrunch"
          url: "https://techcrunch.com/feed/"
        - name: "Ars Technica"
          url: "https://feeds.arstechnica.com/arstechnica/index"
        - name: "The Verge"
          url: "https://www.theverge.com/rss/index.xml"

    twitter:
      enabled: false  # 默认禁用，成本高
      api_provider: "twitterapi.io"

  # 过滤配置
  filter:
    keywords:
      include:
        - "AI"
        - "LLM"
        - "GPT"
        - "Claude"
        - "machine learning"
        - "neural network"
        - "Rust"
        - "Go"
        - "TypeScript"
      exclude:
        - "crypto"
        - "NFT"
        - "blockchain"

    dedup:
      enabled: true
      similarity_threshold: 0.8

  # AI 配置
  summarizer:
    model: "claude-3-haiku"  # 成本优化
    max_daily_cost: 0.50
    importance_threshold: 6.0  # 只处理重要性 >6 的

  # 存储配置
  storage:
    type: "markdown"  # 或 "sqlite"
    retention_days: 30
    archive_compress: true

  # 调度配置
  schedule:
    collection_interval: "30m"
    report_time: "08:00"
    timezone: "Asia/Shanghai"
```

---

## 6. 测试策略

### 单元测试

```go
// internal/collector/hackernews/collector_test.go

package hackernews_test

import (
    "context"
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/anthropic/autonomous-runner/internal/collector"
    "github.com/anthropic/autonomous-runner/internal/collector/hackernews"
)

func TestCollector_Collect(t *testing.T) {
    // 使用真实的测试服务器，不是 mock
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        switch r.URL.Path {
        case "/v0/topstories.json":
            w.Write([]byte(`[1, 2, 3]`))
        case "/v0/item/1.json":
            w.Write([]byte(`{"id":1,"title":"Test","url":"http://test.com","score":100}`))
        // ... 其他 case
        }
    }))
    defer server.Close()

    c := hackernews.NewWithBaseURL("top", server.URL)

    articles, err := c.Collect(context.Background(), collector.Options{
        Limit: 3,
    })

    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }

    if len(articles) != 3 {
        t.Errorf("expected 3 articles, got %d", len(articles))
    }
}
```

### 集成测试

```go
// internal/integration/integration_test.go

package integration_test

import (
    "context"
    "os"
    "testing"
    "time"

    "github.com/anthropic/autonomous-runner/internal/collector"
    "github.com/anthropic/autonomous-runner/internal/collector/hackernews"
    "github.com/anthropic/autonomous-runner/internal/filter"
    "github.com/anthropic/autonomous-runner/internal/storage"
)

func TestEndToEnd(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }

    // 1. 设置临时目录
    tmpDir, _ := os.MkdirTemp("", "techpulse-test")
    defer os.RemoveAll(tmpDir)

    // 2. 创建收集器
    registry := collector.NewRegistry()
    registry.Register(hackernews.New("top"))

    // 3. 收集
    results := registry.CollectAll(context.Background(), collector.Options{
        Limit:   10,
        Timeout: 30 * time.Second,
    })

    // 4. 过滤
    pipeline := filter.NewPipeline(
        filter.NewKeywordFilter([]string{"AI"}, nil),
        filter.NewDedupFilter(0.8),
    )

    for _, r := range results {
        if r.Error != nil {
            continue
        }
        filtered := pipeline.Process(r.Articles)

        // 5. 存储
        store := storage.NewMarkdownStorage(tmpDir)
        // ... 验证存储

        if len(filtered) == 0 {
            t.Log("No articles matched filters")
        }
    }
}
```

---

## 7. 架构检查清单

### 实现前

- [x] 需求分析完成
- [x] 代码量估算 (每文件 <200 行)
- [x] 文件结构设计
- [x] 接口定义
- [x] 依赖关系映射

### 实现中

- [ ] 每个文件 <200 行
- [ ] 单一职责
- [ ] 依赖注入
- [ ] 错误处理完整
- [ ] 无硬编码值

### 测试

- [ ] 使用真实实现
- [ ] 无 mock (核心逻辑)
- [ ] 边界情况覆盖
- [ ] 集成测试存在

### 审查

- [ ] 架构文档更新
- [ ] 公共 API 清晰
- [ ] 无循环依赖
- [ ] 易于扩展

---

## 变更日志

| 版本 | 日期 | 变更内容 |
|------|------|----------|
| v1.0 | 2026-01-01 | 初始架构设计 |
