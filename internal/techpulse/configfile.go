// Configuration file support for TechPulse.
package techpulse

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// FileConfig represents the YAML configuration file structure.
type FileConfig struct {
	Limit         int             `yaml:"limit"`
	Output        string          `yaml:"output"`
	Timeout       int             `yaml:"timeout"`
	RetentionDays *int            `yaml:"retention_days"` // Pointer so 0 (disable) is distinct from unset
	Sources       []string        `yaml:"sources"`
	Keywords      KeywordsConfig  `yaml:"keywords"`
	RSSFeeds      []RSSFeedConfig `yaml:"rss_feeds"`
	EnableSummary *bool           `yaml:"enable_summary"` // Pointer to distinguish unset from false
}

// KeywordsConfig contains keyword filter settings.
type KeywordsConfig struct {
	Include []string `yaml:"include"`
	Exclude []string `yaml:"exclude"`
}

// RSSFeedConfig represents a custom RSS feed.
type RSSFeedConfig struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
}

// LoadConfigFile reads configuration from a YAML file.
func LoadConfigFile(path string) (*FileConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg FileConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// FindConfigFile looks for config file in standard locations.
func FindConfigFile() string {
	paths := []string{
		"techpulse.yaml",
		"techpulse.yml",
		".techpulse/config.yaml",
		".techpulse/config.yml",
	}

	// Check home directory
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths,
			filepath.Join(home, ".config", "techpulse", "config.yaml"),
			filepath.Join(home, ".config", "techpulse", "config.yml"),
		)
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	return ""
}

// MergeWithConfig merges file config into a base config: any value present in
// the file replaces the corresponding default in base, so callers should pass
// a DefaultConfig()-derived base. CLI flag precedence is handled by the
// caller (cmd/techpulse), which layers explicitly-passed flags on top of the
// merged result — that is the only reliable way to distinguish a flag left at
// its default from one the user actually passed.
func MergeWithConfig(base Config, file *FileConfig) Config {
	if file == nil {
		return base
	}

	result := base

	// File values replace defaults in base wherever the file specifies one.
	if file.Limit > 0 && base.Limit == DefaultConfig().Limit {
		result.Limit = file.Limit
	}
	if file.Output != "" && base.Output == DefaultConfig().Output {
		result.Output = file.Output
	}
	if file.Timeout > 0 && base.Timeout == DefaultConfig().Timeout {
		result.Timeout = file.Timeout
	}
	if file.RetentionDays != nil {
		result.RetentionDays = *file.RetentionDays
	}
	if len(file.Sources) > 0 && len(base.Sources) == 0 {
		result.Sources = file.Sources
	}

	// Store custom settings for later use
	result.Keywords = &file.Keywords
	result.RSSFeeds = file.RSSFeeds

	// EnableSummary is a pointer in FileConfig, so an explicit file value
	// always replaces the default here; explicit CLI --summary values are
	// layered on top afterwards by the caller.
	if file.EnableSummary != nil && !base.EnableSummary {
		result.EnableSummary = *file.EnableSummary
	}

	return result
}

// sortedSourceIDs returns all valid source IDs in sorted order so generated
// output and help text are deterministic despite ValidSources being a map.
func sortedSourceIDs() []string {
	ids := make([]string, 0, len(ValidSources))
	for id := range ValidSources {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// GenerateExampleConfig creates an example config file.
func GenerateExampleConfig(path string) error {
	example := fmt.Sprintf(`# TechPulse Configuration
# This is an example configuration file.

# Maximum articles per source (default: 30)
limit: 30

# Output directory for reports (default: .techpulse)
output: .techpulse

# Request timeout in seconds (default: 60). Applied per HTTP request.
timeout: 60

# Archive retention in days (default: 30). Set to 0 to disable cleanup.
# retention_days: 30

# Specify which sources to use (optional, uses all if empty)
# Available: %s
# sources:
#   - hackernews_top
#   - rss

# Keyword filters
keywords:
  # Include articles matching these keywords
  include:
    - AI
    - LLM
    - GPT
    - Claude
    - OpenAI
    - Anthropic
    - machine learning
    - neural network
    - deep learning
    - Rust
    - Go
    - TypeScript
    - Python

  # Exclude articles matching these keywords
  exclude:
    - crypto
    - NFT
    - blockchain
    - bitcoin

# Custom RSS feeds (optional, adds to defaults)
# rss_feeds:
#   - name: Custom Feed
#     url: https://example.com/feed.xml

# Enable article content summary extraction (default: false)
# When enabled, extracts first ~200 characters from article content
enable_summary: false
`, strings.Join(sortedSourceIDs(), ", "))

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(path, []byte(example), 0644)
}
