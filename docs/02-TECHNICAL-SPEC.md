# TechPulse - 技术规格文档

**作者**: Claude AI
**状态**: Proposed
**创建日期**: 2026-01-01
**审阅者**: @tech-lead

---

## 1. 概述

### 1.1 问题陈述

**当前状态**: autonomous-runner 项目提供了自主运行的基础框架，但缺少实际的任务执行模块。

**目标**: 新增 Collector 模块，实现从多个数据源自动收集 AI/科技相关信息，与现有架构无缝集成。

**约束条件**:
- 必须与现有 Memory、Worker、Progress 模块协作
- 单文件不超过 200 行代码
- 支持可插拔的数据源架构
- API 调用成本控制在每日 $0.50 以内

### 1.2 成功指标

| 指标 | 当前值 | 目标值 | 测量方式 |
|------|--------|--------|----------|
| 数据源数量 | 0 | 3+ | 支持的 Collector 数量 |
| 收集延迟 | N/A | <5min | 从发布到收集的时间差 |
| 代码覆盖率 | N/A | >80% | go test -cover |
| 集成测试通过 | N/A | 100% | CI 流水线 |

---

## 2. 系统架构

### 2.1 C4 模型 - 上下文图

```
┌─────────────────────────────────────────────────────────────────┐
│                        TechPulse System                         │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────┐                            ┌───────────────┐  │
│  │    User     │ ──reads──────────────────▶ │ Markdown Files│  │
│  │  (Developer)│                            │ (DIGEST.md)   │  │
│  └─────────────┘                            └───────────────┘  │
│         │                                          ▲            │
│         │ runs                                     │ writes     │
│         ▼                                          │            │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │              Autonomous Runner (Orchestrator)            │   │
│  │  ┌─────────┐  ┌─────────┐  ┌──────────┐  ┌───────────┐  │   │
│  │  │ Config  │  │ Memory  │  │ Progress │  │  Worker   │  │   │
│  │  └─────────┘  └─────────┘  └──────────┘  └───────────┘  │   │
│  │                              ▲                           │   │
│  │                              │                           │   │
│  │  ┌───────────────────────────┴───────────────────────┐  │   │
│  │  │                 Collector Module (NEW)             │  │   │
│  │  │  ┌─────┐  ┌────────┐  ┌─────┐  ┌─────────────┐    │  │   │
│  │  │  │ HN  │  │ Reddit │  │ RSS │  │ Twitter(opt)│    │  │   │
│  │  │  └─────┘  └────────┘  └─────┘  └─────────────┘    │  │   │
│  │  └───────────────────────────────────────────────────┘  │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
                              │
                              │ fetches
                              ▼
            ┌─────────────────────────────────────┐
            │         External Data Sources        │
            │  ┌──────────┐  ┌────────┐  ┌─────┐  │
            │  │Hacker    │  │Reddit  │  │ RSS │  │
            │  │News API  │  │API     │  │Feeds│  │
            │  └──────────┘  └────────┘  └─────┘  │
            └─────────────────────────────────────┘
```

### 2.2 C4 模型 - 容器图

```
┌────────────────────────────────────────────────────────────────────┐
│                      Autonomous Runner Container                    │
├────────────────────────────────────────────────────────────────────┤
│                                                                    │
│  ┌──────────────────┐     ┌──────────────────┐                    │
│  │   Orchestrator   │────▶│     Config       │                    │
│  │   (cmd/main.go)  │     │ (internal/config)│                    │
│  └────────┬─────────┘     └──────────────────┘                    │
│           │                                                        │
│           │ coordinates                                            │
│           ▼                                                        │
│  ┌────────────────────────────────────────────────────────────┐   │
│  │                    Core Modules                             │   │
│  │                                                             │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐ │   │
│  │  │   Memory    │  │  Progress   │  │      Worker         │ │   │
│  │  │             │  │             │  │                     │ │   │
│  │  │ - TASKS.md  │  │ - Git Det   │  │ - Claude CLI Exec   │ │   │
│  │  │ - CONTEXT.md│  │ - Hash Det  │  │ - Output Parsing    │ │   │
│  │  │ - DONE.md   │  │             │  │ - Cost Tracking     │ │   │
│  │  └─────────────┘  └─────────────┘  └─────────────────────┘ │   │
│  └────────────────────────────────────────────────────────────┘   │
│                              │                                     │
│                              │ integrates with                     │
│                              ▼                                     │
│  ┌────────────────────────────────────────────────────────────┐   │
│  │               Collector Module (NEW)                        │   │
│  │                                                             │   │
│  │  ┌───────────────┐  ┌───────────────┐  ┌───────────────┐   │   │
│  │  │   Collector   │  │    Filter     │  │  Summarizer   │   │   │
│  │  │   Interface   │  │    Module     │  │    Module     │   │   │
│  │  │               │  │               │  │               │   │   │
│  │  │ - HN Impl     │  │ - Importance  │  │ - AI Summary  │   │   │
│  │  │ - Reddit Impl │  │ - Keywords    │  │ - Trends      │   │   │
│  │  │ - RSS Impl    │  │ - Dedup       │  │               │   │   │
│  │  └───────────────┘  └───────────────┘  └───────────────┘   │   │
│  │                              │                              │   │
│  │                              ▼                              │   │
│  │  ┌───────────────────────────────────────────────────────┐ │   │
│  │  │                   Storage Module                       │ │   │
│  │  │                                                        │ │   │
│  │  │   - Markdown Writer (integrates with Memory)           │ │   │
│  │  │   - Optional: SQLite                                   │ │   │
│  │  └───────────────────────────────────────────────────────┘ │   │
│  └────────────────────────────────────────────────────────────┘   │
│                                                                    │
└────────────────────────────────────────────────────────────────────┘
```

### 2.3 数据流图

```
┌─────────────────────────────────────────────────────────────────────┐
│                         Data Flow Diagram                           │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│   [External Sources]                                                │
│         │                                                           │
│         ▼                                                           │
│   ┌─────────────┐                                                   │
│   │  Collector  │  ──── Collects raw articles ────────┐             │
│   │  Interface  │                                     │             │
│   └─────────────┘                                     │             │
│         │                                             │             │
│         │ []RawArticle                                │             │
│         ▼                                             │             │
│   ┌─────────────┐                                     │             │
│   │   Filter    │  ──── Dedup + Keyword Match ────────┤             │
│   │   Module    │                                     │             │
│   └─────────────┘                                     │             │
│         │                                             │             │
│         │ []FilteredArticle                           │             │
│         ▼                                             │             │
│   ┌─────────────┐                                     │             │
│   │ Summarizer  │  ──── AI Scoring + Summary ─────────┤             │
│   │   Module    │                                     │             │
│   └─────────────┘                                     │             │
│         │                                             │             │
│         │ []EnrichedArticle                           │             │
│         ▼                                             │             │
│   ┌─────────────┐      ┌───────────────────────────┐  │             │
│   │   Storage   │ ───▶ │      DIGEST.md            │  │             │
│   │   Module    │      │      TRENDS.md            │  │             │
│   └─────────────┘      │      archive/YYYY-MM-DD/  │  │             │
│                        └───────────────────────────┘  │             │
│                                     │                 │             │
│                                     ▼                 │             │
│                        ┌───────────────────────────┐  │             │
│                        │    Memory Module          │◀─┘             │
│                        │    (CONTEXT.md update)    │                │
│                        └───────────────────────────┘                │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 3. 详细设计

### 3.1 Collector 接口设计

```go
// internal/collector/collector.go

package collector

import (
    "context"
    "time"
)

// Article 表示从任意数据源收集的文章
type Article struct {
    ID          string            `json:"id"`
    Source      string            `json:"source"`
    SourceID    string            `json:"source_id"`
    Title       string            `json:"title"`
    URL         string            `json:"url"`
    Content     string            `json:"content,omitempty"`
    Author      string            `json:"author,omitempty"`
    Score       int               `json:"score"`
    Comments    int               `json:"comments"`
    Tags        []string          `json:"tags,omitempty"`
    Metadata    map[string]string `json:"metadata,omitempty"`
    PublishedAt time.Time         `json:"published_at"`
    CollectedAt time.Time         `json:"collected_at"`
}

// Collector 定义数据源收集器的接口
type Collector interface {
    // Name 返回收集器名称
    Name() string

    // Collect 执行收集操作，返回文章列表
    Collect(ctx context.Context, opts CollectOptions) ([]Article, error)

    // Validate 验证配置是否有效
    Validate() error
}

// CollectOptions 收集选项
type CollectOptions struct {
    Limit     int           // 最大收集数量
    Since     time.Time     // 收集此时间之后的内容
    Category  string        // 分类筛选
    Timeout   time.Duration // 超时时间
}

// Registry 管理所有注册的收集器
type Registry struct {
    collectors map[string]Collector
}

func NewRegistry() *Registry {
    return &Registry{
        collectors: make(map[string]Collector),
    }
}

func (r *Registry) Register(c Collector) {
    r.collectors[c.Name()] = c
}

func (r *Registry) Get(name string) (Collector, bool) {
    c, ok := r.collectors[name]
    return c, ok
}

func (r *Registry) All() []Collector {
    result := make([]Collector, 0, len(r.collectors))
    for _, c := range r.collectors {
        result = append(result, c)
    }
    return result
}
```

### 3.2 Hacker News 收集器

```go
// internal/collector/hackernews/collector.go

package hackernews

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "time"

    "github.com/anthropic/autonomous-runner/internal/collector"
)

const (
    baseURL = "https://hacker-news.firebaseio.com/v0"
)

type HNCollector struct {
    client   *http.Client
    category string // "top", "new", "best", "ask", "show"
}

func New(category string) *HNCollector {
    return &HNCollector{
        client: &http.Client{
            Timeout: 30 * time.Second,
        },
        category: category,
    }
}

func (c *HNCollector) Name() string {
    return fmt.Sprintf("hackernews_%s", c.category)
}

func (c *HNCollector) Validate() error {
    validCategories := map[string]bool{
        "top": true, "new": true, "best": true,
        "ask": true, "show": true, "job": true,
    }
    if !validCategories[c.category] {
        return fmt.Errorf("invalid category: %s", c.category)
    }
    return nil
}

func (c *HNCollector) Collect(ctx context.Context, opts collector.CollectOptions) ([]collector.Article, error) {
    // 1. 获取 story IDs
    ids, err := c.fetchStoryIDs(ctx)
    if err != nil {
        return nil, fmt.Errorf("fetch story IDs: %w", err)
    }

    // 2. 限制数量
    limit := opts.Limit
    if limit <= 0 || limit > len(ids) {
        limit = len(ids)
    }
    if limit > 50 {
        limit = 50 // 默认最大 50
    }
    ids = ids[:limit]

    // 3. 并发获取详情
    articles := make([]collector.Article, 0, len(ids))
    for _, id := range ids {
        item, err := c.fetchItem(ctx, id)
        if err != nil {
            continue // 跳过失败的
        }

        // 时间过滤
        publishedAt := time.Unix(int64(item.Time), 0)
        if !opts.Since.IsZero() && publishedAt.Before(opts.Since) {
            continue
        }

        articles = append(articles, collector.Article{
            ID:          fmt.Sprintf("hn_%d", item.ID),
            Source:      "hackernews",
            SourceID:    fmt.Sprintf("%d", item.ID),
            Title:       item.Title,
            URL:         item.URL,
            Author:      item.By,
            Score:       item.Score,
            Comments:    item.Descendants,
            PublishedAt: publishedAt,
            CollectedAt: time.Now(),
            Metadata: map[string]string{
                "category": c.category,
                "type":     item.Type,
            },
        })
    }

    return articles, nil
}

// HN API 数据结构
type hnItem struct {
    ID          int    `json:"id"`
    Type        string `json:"type"`
    By          string `json:"by"`
    Time        int    `json:"time"`
    Title       string `json:"title"`
    URL         string `json:"url"`
    Score       int    `json:"score"`
    Descendants int    `json:"descendants"`
}

func (c *HNCollector) fetchStoryIDs(ctx context.Context) ([]int, error) {
    url := fmt.Sprintf("%s/%sstories.json", baseURL, c.category)

    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return nil, err
    }

    resp, err := c.client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var ids []int
    if err := json.NewDecoder(resp.Body).Decode(&ids); err != nil {
        return nil, err
    }

    return ids, nil
}

func (c *HNCollector) fetchItem(ctx context.Context, id int) (*hnItem, error) {
    url := fmt.Sprintf("%s/item/%d.json", baseURL, id)

    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return nil, err
    }

    resp, err := c.client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var item hnItem
    if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
        return nil, err
    }

    return &item, nil
}
```

### 3.3 Filter 模块设计

```go
// internal/filter/filter.go

package filter

import (
    "crypto/sha256"
    "encoding/hex"
    "regexp"
    "strings"

    "github.com/anthropic/autonomous-runner/internal/collector"
)

// FilteredArticle 过滤后的文章，包含匹配信息
type FilteredArticle struct {
    collector.Article
    MatchedKeywords []string `json:"matched_keywords"`
    IsDuplicate     bool     `json:"is_duplicate"`
    DuplicateOf     string   `json:"duplicate_of,omitempty"`
}

// Filter 定义过滤器接口
type Filter interface {
    Apply(articles []collector.Article) []FilteredArticle
}

// KeywordFilter 关键词过滤器
type KeywordFilter struct {
    Include  []string         // 包含关键词
    Exclude  []string         // 排除关键词
    patterns []*regexp.Regexp // 预编译的正则
}

func NewKeywordFilter(include, exclude []string) *KeywordFilter {
    f := &KeywordFilter{
        Include: include,
        Exclude: exclude,
    }

    // 预编译正则表达式
    for _, kw := range include {
        if pattern, err := regexp.Compile("(?i)" + regexp.QuoteMeta(kw)); err == nil {
            f.patterns = append(f.patterns, pattern)
        }
    }

    return f
}

func (f *KeywordFilter) Apply(articles []collector.Article) []FilteredArticle {
    result := make([]FilteredArticle, 0, len(articles))

    for _, a := range articles {
        matched := f.matchKeywords(a)
        if len(f.Include) > 0 && len(matched) == 0 {
            continue // 必须匹配至少一个包含关键词
        }

        if f.matchExclude(a) {
            continue // 匹配排除关键词，跳过
        }

        result = append(result, FilteredArticle{
            Article:         a,
            MatchedKeywords: matched,
        })
    }

    return result
}

func (f *KeywordFilter) matchKeywords(a collector.Article) []string {
    text := strings.ToLower(a.Title + " " + a.Content)
    var matched []string

    for i, pattern := range f.patterns {
        if pattern.MatchString(text) {
            matched = append(matched, f.Include[i])
        }
    }

    return matched
}

func (f *KeywordFilter) matchExclude(a collector.Article) bool {
    text := strings.ToLower(a.Title + " " + a.Content)

    for _, kw := range f.Exclude {
        if strings.Contains(text, strings.ToLower(kw)) {
            return true
        }
    }

    return false
}

// DedupFilter 去重过滤器
type DedupFilter struct {
    seen       map[string]string // hash -> article ID
    similarity float64           // 相似度阈值
}

func NewDedupFilter(similarity float64) *DedupFilter {
    return &DedupFilter{
        seen:       make(map[string]string),
        similarity: similarity,
    }
}

func (f *DedupFilter) Apply(articles []FilteredArticle) []FilteredArticle {
    result := make([]FilteredArticle, 0, len(articles))

    for _, a := range articles {
        hash := f.computeHash(a.Article)

        if existingID, exists := f.seen[hash]; exists {
            a.IsDuplicate = true
            a.DuplicateOf = existingID
        } else {
            f.seen[hash] = a.ID
        }

        if !a.IsDuplicate {
            result = append(result, a)
        }
    }

    return result
}

func (f *DedupFilter) computeHash(a collector.Article) string {
    // 使用标题的规范化形式计算哈希
    normalized := strings.ToLower(strings.TrimSpace(a.Title))
    normalized = regexp.MustCompile(`\s+`).ReplaceAllString(normalized, " ")

    hash := sha256.Sum256([]byte(normalized))
    return hex.EncodeToString(hash[:8]) // 使用前 8 字节
}
```

### 3.4 Summarizer 模块设计

```go
// internal/summarizer/summarizer.go

package summarizer

import (
    "context"
    "fmt"

    "github.com/anthropic/autonomous-runner/internal/filter"
)

// EnrichedArticle 带有 AI 增强信息的文章
type EnrichedArticle struct {
    filter.FilteredArticle
    Importance float64  `json:"importance"` // 0-10 评分
    Summary    string   `json:"summary"`
    AITags     []string `json:"ai_tags"`
}

// Summarizer AI 摘要和评分接口
type Summarizer interface {
    // Enrich 对文章进行 AI 增强处理
    Enrich(ctx context.Context, articles []filter.FilteredArticle) ([]EnrichedArticle, error)

    // GenerateDailyReport 生成每日报告
    GenerateDailyReport(ctx context.Context, articles []EnrichedArticle) (string, error)
}

// ClaudeSummarizer 使用 Claude 进行摘要
type ClaudeSummarizer struct {
    model      string
    maxCost    float64
    currentCost float64
}

func NewClaudeSummarizer(model string, maxCost float64) *ClaudeSummarizer {
    return &ClaudeSummarizer{
        model:   model,
        maxCost: maxCost,
    }
}

func (s *ClaudeSummarizer) Enrich(ctx context.Context, articles []filter.FilteredArticle) ([]EnrichedArticle, error) {
    result := make([]EnrichedArticle, 0, len(articles))

    for _, a := range articles {
        // 成本控制
        if s.currentCost >= s.maxCost {
            // 超过成本限制，使用基础评分
            result = append(result, EnrichedArticle{
                FilteredArticle: a,
                Importance:      s.basicImportanceScore(a),
                Summary:         "",
                AITags:          []string{},
            })
            continue
        }

        // AI 评分和摘要
        enriched, cost, err := s.enrichWithAI(ctx, a)
        if err != nil {
            // 失败时使用基础评分
            result = append(result, EnrichedArticle{
                FilteredArticle: a,
                Importance:      s.basicImportanceScore(a),
            })
            continue
        }

        s.currentCost += cost
        result = append(result, enriched)
    }

    return result, nil
}

func (s *ClaudeSummarizer) basicImportanceScore(a filter.FilteredArticle) float64 {
    // 基于元数据的基础评分
    score := 5.0 // 基础分

    // 根据评分调整
    if a.Score > 500 {
        score += 2
    } else if a.Score > 100 {
        score += 1
    }

    // 根据评论数调整
    if a.Comments > 200 {
        score += 1.5
    } else if a.Comments > 50 {
        score += 0.5
    }

    // 根据匹配关键词数量
    score += float64(len(a.MatchedKeywords)) * 0.5

    if score > 10 {
        score = 10
    }

    return score
}

func (s *ClaudeSummarizer) enrichWithAI(ctx context.Context, a filter.FilteredArticle) (EnrichedArticle, float64, error) {
    // TODO: 调用 Claude API 进行实际的 AI 处理
    // 这里返回模拟结果
    return EnrichedArticle{
        FilteredArticle: a,
        Importance:      s.basicImportanceScore(a),
        Summary:         fmt.Sprintf("Summary of: %s", a.Title),
        AITags:          []string{"tech", "ai"},
    }, 0.001, nil
}

func (s *ClaudeSummarizer) GenerateDailyReport(ctx context.Context, articles []EnrichedArticle) (string, error) {
    // 按重要性排序
    // 生成 Markdown 报告
    report := "# Daily Tech Digest\n\n"
    report += fmt.Sprintf("## Top Stories (%d articles)\n\n", len(articles))

    for i, a := range articles {
        if i >= 10 {
            break
        }
        report += fmt.Sprintf("### %d. %s\n", i+1, a.Title)
        report += fmt.Sprintf("- Source: %s | Score: %d | Importance: %.1f/10\n",
            a.Source, a.Score, a.Importance)
        report += fmt.Sprintf("- [Read more](%s)\n\n", a.URL)
    }

    return report, nil
}
```

### 3.5 Storage 模块设计

```go
// internal/storage/storage.go

package storage

import (
    "fmt"
    "os"
    "path/filepath"
    "time"

    "github.com/anthropic/autonomous-runner/internal/summarizer"
)

// Storage 存储接口
type Storage interface {
    // Save 保存文章列表
    Save(articles []summarizer.EnrichedArticle) error

    // SaveReport 保存每日报告
    SaveReport(report string) error

    // Load 加载历史文章
    Load(since time.Time) ([]summarizer.EnrichedArticle, error)
}

// MarkdownStorage Markdown 文件存储
type MarkdownStorage struct {
    baseDir     string
    digestFile  string
    archiveDir  string
}

func NewMarkdownStorage(baseDir string) *MarkdownStorage {
    return &MarkdownStorage{
        baseDir:    baseDir,
        digestFile: filepath.Join(baseDir, "DIGEST.md"),
        archiveDir: filepath.Join(baseDir, "archive"),
    }
}

func (s *MarkdownStorage) Save(articles []summarizer.EnrichedArticle) error {
    // 确保目录存在
    if err := os.MkdirAll(s.archiveDir, 0755); err != nil {
        return fmt.Errorf("create archive dir: %w", err)
    }

    // 按日期归档
    today := time.Now().Format("2006-01-02")
    archiveFile := filepath.Join(s.archiveDir, today+".md")

    // 生成 Markdown 内容
    content := s.articlesToMarkdown(articles)

    // 写入归档文件
    if err := os.WriteFile(archiveFile, []byte(content), 0644); err != nil {
        return fmt.Errorf("write archive: %w", err)
    }

    return nil
}

func (s *MarkdownStorage) SaveReport(report string) error {
    return os.WriteFile(s.digestFile, []byte(report), 0644)
}

func (s *MarkdownStorage) articlesToMarkdown(articles []summarizer.EnrichedArticle) string {
    md := fmt.Sprintf("# Tech Digest - %s\n\n", time.Now().Format("2006-01-02"))
    md += fmt.Sprintf("Collected %d articles\n\n", len(articles))

    // 按来源分组
    bySource := make(map[string][]summarizer.EnrichedArticle)
    for _, a := range articles {
        bySource[a.Source] = append(bySource[a.Source], a)
    }

    for source, sourceArticles := range bySource {
        md += fmt.Sprintf("## %s (%d)\n\n", source, len(sourceArticles))

        for _, a := range sourceArticles {
            md += fmt.Sprintf("### [%s](%s)\n", a.Title, a.URL)
            md += fmt.Sprintf("- Score: %d | Comments: %d | Importance: %.1f/10\n",
                a.Score, a.Comments, a.Importance)
            if a.Summary != "" {
                md += fmt.Sprintf("- Summary: %s\n", a.Summary)
            }
            if len(a.MatchedKeywords) > 0 {
                md += fmt.Sprintf("- Keywords: %v\n", a.MatchedKeywords)
            }
            md += "\n"
        }
    }

    return md
}

func (s *MarkdownStorage) Load(since time.Time) ([]summarizer.EnrichedArticle, error) {
    // TODO: 实现从归档文件加载
    return nil, nil
}
```

---

## 4. 与现有模块集成

### 4.1 与 Memory 模块集成

```go
// internal/collector/integration.go

package collector

import (
    "fmt"
    "strings"
    "time"

    "github.com/anthropic/autonomous-runner/internal/memory"
    "github.com/anthropic/autonomous-runner/internal/summarizer"
)

// MemoryIntegration 与 Memory 模块的集成
type MemoryIntegration struct {
    memoryManager *memory.Manager
}

func NewMemoryIntegration(mm *memory.Manager) *MemoryIntegration {
    return &MemoryIntegration{
        memoryManager: mm,
    }
}

// UpdateContext 更新 CONTEXT.md 中的收集状态
func (m *MemoryIntegration) UpdateContext(articles []summarizer.EnrichedArticle) error {
    // 生成收集摘要
    summary := m.generateCollectionSummary(articles)

    // 更新 CONTEXT.md
    // 这会触发 Memory 模块的变更检测
    return m.memoryManager.UpdateContext(summary)
}

// AddCollectionTask 添加收集任务到 TASKS.md
func (m *MemoryIntegration) AddCollectionTask(source string, interval time.Duration) error {
    task := fmt.Sprintf("- [ ] Collect from %s (every %v)", source, interval)
    return m.memoryManager.AddTask(task)
}

// MarkCollectionDone 标记收集完成到 DONE.md
func (m *MemoryIntegration) MarkCollectionDone(source string, count int) error {
    entry := fmt.Sprintf("| %s | Collected %d articles from %s | %s |",
        time.Now().Format("2006-01-02 15:04"),
        count,
        source,
        time.Now().Format(time.RFC3339),
    )
    return m.memoryManager.AddDoneEntry(entry)
}

func (m *MemoryIntegration) generateCollectionSummary(articles []summarizer.EnrichedArticle) string {
    var sb strings.Builder

    sb.WriteString("## Latest Collection\n\n")
    sb.WriteString(fmt.Sprintf("- Total articles: %d\n", len(articles)))

    // 按来源统计
    bySource := make(map[string]int)
    for _, a := range articles {
        bySource[a.Source]++
    }

    sb.WriteString("- By source:\n")
    for source, count := range bySource {
        sb.WriteString(fmt.Sprintf("  - %s: %d\n", source, count))
    }

    // Top 5 重要文章
    sb.WriteString("\n### Top 5 Important\n")
    for i, a := range articles {
        if i >= 5 {
            break
        }
        sb.WriteString(fmt.Sprintf("1. [%.1f] %s\n", a.Importance, a.Title))
    }

    return sb.String()
}
```

### 4.2 与 Worker 模块集成

```go
// internal/collector/worker_integration.go

package collector

import (
    "context"
    "fmt"

    "github.com/anthropic/autonomous-runner/internal/worker"
)

// WorkerTask 定义收集任务供 Worker 执行
type WorkerTask struct {
    Name        string
    Description string
    Execute     func(ctx context.Context) error
}

// CreateCollectionTask 创建一个收集任务
func CreateCollectionTask(
    registry *Registry,
    opts CollectOptions,
) WorkerTask {
    return WorkerTask{
        Name:        "tech_pulse_collection",
        Description: "Collect AI/Tech news from multiple sources",
        Execute: func(ctx context.Context) error {
            var allArticles []Article

            // 从所有注册的收集器收集
            for _, c := range registry.All() {
                articles, err := c.Collect(ctx, opts)
                if err != nil {
                    // 记录错误但继续
                    fmt.Printf("Error collecting from %s: %v\n", c.Name(), err)
                    continue
                }
                allArticles = append(allArticles, articles...)
            }

            fmt.Printf("Collected %d articles total\n", len(allArticles))
            return nil
        },
    }
}

// RegisterWithWorker 将收集任务注册到 Worker
func RegisterWithWorker(w *worker.Runner, task WorkerTask) {
    // Worker 会在每次迭代时执行这个任务
    w.RegisterTask(task.Name, task.Description, task.Execute)
}
```

### 4.3 与 Progress 模块集成

```go
// internal/collector/progress_integration.go

package collector

import (
    "crypto/md5"
    "encoding/hex"
    "fmt"

    "github.com/anthropic/autonomous-runner/internal/progress"
)

// CollectionDetector 检测收集进度的检测器
type CollectionDetector struct {
    lastDigestHash string
    articleCount   int
}

func NewCollectionDetector() *CollectionDetector {
    return &CollectionDetector{}
}

// 实现 progress.Detector 接口

func (d *CollectionDetector) Name() string {
    return "collection"
}

func (d *CollectionDetector) Detect() (bool, error) {
    // 检查 DIGEST.md 是否有变化
    currentHash, err := d.computeDigestHash()
    if err != nil {
        return false, err
    }

    if currentHash != d.lastDigestHash {
        d.lastDigestHash = currentHash
        return true, nil // 检测到进度
    }

    return false, nil
}

func (d *CollectionDetector) Reset() {
    d.lastDigestHash = ""
    d.articleCount = 0
}

func (d *CollectionDetector) Details() string {
    return fmt.Sprintf("Articles collected: %d", d.articleCount)
}

func (d *CollectionDetector) computeDigestHash() (string, error) {
    // 计算 DIGEST.md 的哈希
    // 简化实现
    hash := md5.Sum([]byte(fmt.Sprintf("%d", d.articleCount)))
    return hex.EncodeToString(hash[:]), nil
}

// RegisterCollectionDetector 注册收集检测器
func RegisterCollectionDetector(multi *progress.MultiDetector) {
    multi.Register(NewCollectionDetector())
}
```

---

## 5. 替代方案对比

### 5.1 数据存储方案

| 方案 | 优点 | 缺点 | 决策 |
|------|------|------|------|
| **Markdown 文件 (推荐)** | 人类可读、Git 友好、与现有 Memory 兼容 | 查询能力有限 | **采用** - 与现有架构一致 |
| SQLite | 结构化查询、关系支持 | 需要额外依赖、不可读 | 备选 - 可选添加 |
| JSON 文件 | 机器可读、易解析 | 不如 Markdown 可读 | 拒绝 |

### 5.2 AI 调用方案

| 方案 | 优点 | 缺点 | 决策 |
|------|------|------|------|
| **Claude CLI (推荐)** | 与现有 Worker 一致、成本可控 | 延迟较高 | **采用** |
| OpenAI API | 生态丰富 | 需要额外集成 | 备选 |
| 本地模型 | 无 API 成本 | 质量不足、资源占用 | 拒绝 |

### 5.3 Twitter 数据源方案

| 方案 | 优点 | 缺点 | 决策 |
|------|------|------|------|
| 官方 API | 官方支持 | $100+/月，太贵 | 拒绝 |
| **第三方 API (推荐)** | 成本低 ($0.15/1K) | 可能不稳定 | **备选** - v1.2 考虑 |
| RSS 替代 | 免费 | 覆盖不全 | 临时方案 |
| 不支持 | 无成本 | 缺少数据源 | 当前决策 |

---

## 6. 风险与缓解

| 风险 | 严重性 | 可能性 | 缓解措施 |
|------|--------|--------|----------|
| Reddit API 限流 | 高 | 高 | 实现指数退避、缓存、队列 |
| AI 成本超支 | 中 | 中 | 设置每日成本上限、降级策略 |
| 数据源格式变化 | 中 | 中 | 抽象解析层、版本化适配器 |
| 磁盘空间不足 | 低 | 低 | 自动归档压缩、保留期限 |
| 网络不稳定 | 低 | 高 | 重试机制、离线缓存 |

---

## 7. 实现计划

### Phase 1: 核心收集器 (Week 1)
- [x] 定义 Collector 接口
- [ ] 实现 HN Collector
- [ ] 实现 RSS Collector
- [ ] 基础测试

### Phase 2: 过滤与存储 (Week 2)
- [ ] 实现 Filter 模块
- [ ] 实现 Storage 模块
- [ ] Memory 集成

### Phase 3: AI 增强 (Week 3)
- [ ] 实现 Summarizer
- [ ] 重要性评分
- [ ] 每日报告

### Phase 4: 完整集成 (Week 4)
- [ ] Worker 集成
- [ ] Progress 集成
- [ ] 端到端测试

---

## 8. 开放问题

1. **问题**: AI 评分应该在收集时还是定期批量处理？
   - **建议**: 批量处理，减少 API 调用

2. **问题**: 是否需要支持自定义 RSS 源？
   - **影响**: 配置复杂度
   - **建议**: v1.1 支持

3. **问题**: 归档数据保留多久？
   - **建议**: 默认 30 天，可配置

---

## 变更日志

| 版本 | 日期 | 变更内容 |
|------|------|----------|
| v1.0 | 2026-01-01 | 初始技术规格 |
