package techpulse

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/anthropic/autonomous-runner/internal/collector/rss"
)

func TestLoadConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.yaml")

	configContent := `
limit: 50
output: /custom/output
timeout: 120
enable_summary: true
sources:
  - hackernews_top
keywords:
  include: [Kubernetes, Docker]
  exclude: [spam]
rss_feeds:
  - name: Custom Feed
    url: https://example.com/feed.xml
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	cfg, err := LoadConfigFile(configPath)
	if err != nil {
		t.Fatalf("LoadConfigFile failed: %v", err)
	}
	if cfg.Limit != 50 {
		t.Errorf("Limit = %d, want 50", cfg.Limit)
	}
	if cfg.EnableSummary == nil || !*cfg.EnableSummary {
		t.Error("EnableSummary should be true")
	}
	if len(cfg.Sources) != 1 || cfg.Sources[0] != "hackernews_top" {
		t.Errorf("Sources = %v, want [hackernews_top]", cfg.Sources)
	}
	if len(cfg.Keywords.Include) != 2 {
		t.Errorf("Keywords.Include = %v, want 2 items", cfg.Keywords.Include)
	}
}

func TestLoadConfigFileNotFound(t *testing.T) {
	_, err := LoadConfigFile("/nonexistent/config.yaml")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestFindConfigFile(t *testing.T) {
	// Test when no config file exists
	// Save current directory and change to temp
	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// No config should be found
	if path := FindConfigFile(); path != "" {
		t.Errorf("FindConfigFile = %s, want empty", path)
	}

	// Create techpulse.yaml
	if err := os.WriteFile("techpulse.yaml", []byte("limit: 10"), 0644); err != nil {
		t.Fatal(err)
	}

	if path := FindConfigFile(); path != "techpulse.yaml" {
		t.Errorf("FindConfigFile = %s, want techpulse.yaml", path)
	}
}

func TestMergeWithConfig(t *testing.T) {
	base := DefaultConfig()
	enableSummary := true
	retention := 0
	file := &FileConfig{
		Limit:         100,
		Output:        "/custom",
		Timeout:       180,
		RetentionDays: &retention,
		EnableSummary: &enableSummary,
		Keywords:      KeywordsConfig{Include: []string{"custom"}},
	}

	result := MergeWithConfig(base, file)

	if result.Limit != 100 {
		t.Errorf("Merged Limit = %d, want 100", result.Limit)
	}
	if result.Output != "/custom" {
		t.Errorf("Merged Output = %s, want /custom", result.Output)
	}
	if result.RetentionDays != 0 {
		t.Errorf("Merged RetentionDays = %d, want 0", result.RetentionDays)
	}
	if result.Keywords == nil || len(result.Keywords.Include) != 1 {
		t.Error("Keywords not merged correctly")
	}
	if !result.EnableSummary {
		t.Error("EnableSummary should be true from file config")
	}
}

func TestMergeWithConfigNil(t *testing.T) {
	base := DefaultConfig()
	result := MergeWithConfig(base, nil)

	if result.Limit != base.Limit {
		t.Error("Nil merge should return base config")
	}
}

func TestMergeWithConfigCLIPrecedence(t *testing.T) {
	base := DefaultConfig()
	base.Limit = 999          // CLI set this
	base.EnableSummary = true // CLI set --summary

	fileSummary := false
	file := &FileConfig{
		Limit:         50,           // File tries to set different value
		EnableSummary: &fileSummary, // File tries to disable summary
	}

	result := MergeWithConfig(base, file)

	if result.Limit != 999 {
		t.Errorf("CLI should take precedence: got %d, want 999", result.Limit)
	}
	if !result.EnableSummary {
		t.Error("CLI --summary should take precedence over file config")
	}
}

func TestGenerateExampleConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "subdir", "config.yaml")

	if err := GenerateExampleConfig(configPath); err != nil {
		t.Fatalf("GenerateExampleConfig failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(configPath); err != nil {
		t.Errorf("Config file not created: %v", err)
	}

	// Verify it's valid YAML by loading it
	cfg, err := LoadConfigFile(configPath)
	if err != nil {
		t.Errorf("Generated config is invalid YAML: %v", err)
	}
	if cfg.Limit != 30 {
		t.Errorf("Generated config limit = %d, want 30", cfg.Limit)
	}
}

func TestBuildRSSCollector(t *testing.T) {
	// Empty custom feeds should use defaults
	c := buildRSSCollector(nil)
	if c == nil {
		t.Error("buildRSSCollector returned nil")
	}
	if err := c.Validate(); err != nil {
		t.Errorf("default collector Validate() error: %v", err)
	}

	// Custom feeds append to defaults (not replace)
	custom := []RSSFeedConfig{
		{Name: "Test", URL: "https://test.com/feed"},
	}
	c = buildRSSCollector(custom)
	if c == nil {
		t.Error("buildRSSCollector with custom feeds returned nil")
	}
	if err := c.Validate(); err != nil {
		t.Errorf("custom collector Validate() error: %v", err)
	}

	merged := mergeRSSSources(custom)
	wantLen := len(rss.DefaultSources) + 1
	if len(merged) != wantLen {
		t.Fatalf("mergeRSSSources len = %d, want %d", len(merged), wantLen)
	}
	for i, def := range rss.DefaultSources {
		if merged[i].Name != def.Name || merged[i].URL != def.URL {
			t.Errorf("merged[%d] = %+v, want default %+v", i, merged[i], def)
		}
	}
	last := merged[len(merged)-1]
	if last.Name != "Test" || last.URL != "https://test.com/feed" {
		t.Errorf("custom feed not appended: got %+v", last)
	}
}

func TestMergeRSSSourcesEmpty(t *testing.T) {
	merged := mergeRSSSources(nil)
	if len(merged) != len(rss.DefaultSources) {
		t.Fatalf("mergeRSSSources(nil) len = %d, want %d", len(merged), len(rss.DefaultSources))
	}
	merged = mergeRSSSources([]RSSFeedConfig{})
	if len(merged) != len(rss.DefaultSources) {
		t.Fatalf("mergeRSSSources([]) len = %d, want %d", len(merged), len(rss.DefaultSources))
	}
}

func TestBuildFilterPipeline(t *testing.T) {
	// Nil keywords should use defaults
	p := buildFilterPipeline(nil)
	if p == nil {
		t.Error("buildFilterPipeline returned nil")
	}

	// Empty keywords should use defaults
	p = buildFilterPipeline(&KeywordsConfig{})
	if p == nil {
		t.Error("buildFilterPipeline with empty keywords returned nil")
	}

	// Custom keywords
	p = buildFilterPipeline(&KeywordsConfig{
		Include: []string{"test"},
	})
	if p == nil {
		t.Error("buildFilterPipeline with custom keywords returned nil")
	}
}
