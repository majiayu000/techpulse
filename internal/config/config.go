// Package config 提供配置管理功能
package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 运行配置
type Config struct {
	// 限制条件
	MaxIterations        int           `yaml:"max_iterations"`
	MaxCostUSD           float64       `yaml:"max_cost_usd"`
	MaxDuration          time.Duration `yaml:"max_duration"`
	ConsecutiveNoProgress int          `yaml:"consecutive_no_progress"`
	StopWhenEmpty        bool          `yaml:"stop_when_empty"`

	// 执行参数
	CooldownDuration time.Duration `yaml:"cooldown_duration"`
	WorkerTimeout    time.Duration `yaml:"worker_timeout"`

	// 路径
	MemoryDir   string `yaml:"memory_dir"`
	WorkspaceDir string `yaml:"workspace_dir"`
	LogDir      string `yaml:"log_dir"`

	// 进展检测
	UseGitDetection bool `yaml:"use_git_detection"`
}

// Default 返回默认配置
func Default() *Config {
	return &Config{
		MaxIterations:         100,
		MaxCostUSD:            50.0,
		MaxDuration:           8 * time.Hour,
		ConsecutiveNoProgress: 3,
		StopWhenEmpty:         true,
		CooldownDuration:      10 * time.Second,
		WorkerTimeout:         30 * time.Minute,
		MemoryDir:             "memory",
		WorkspaceDir:          "workspace",
		LogDir:                "logs",
		UseGitDetection:       true,
	}
}

// Load 从 YAML 文件加载配置
func Load(path string) (*Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil // 文件不存在，使用默认配置
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate 验证配置有效性
func (c *Config) Validate() error {
	if c.MaxIterations < 0 {
		c.MaxIterations = 0 // 0 表示无限
	}
	if c.MaxCostUSD < 0 {
		c.MaxCostUSD = 0
	}
	if c.WorkerTimeout < time.Minute {
		c.WorkerTimeout = 30 * time.Minute
	}
	return nil
}
