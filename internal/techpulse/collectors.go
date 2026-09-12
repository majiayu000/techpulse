package techpulse

import (
	"github.com/anthropic/autonomous-runner/internal/collector"
	"github.com/anthropic/autonomous-runner/internal/collector/github"
	"github.com/anthropic/autonomous-runner/internal/collector/hackernews"
	"github.com/anthropic/autonomous-runner/internal/collector/lobsters"
	"github.com/anthropic/autonomous-runner/internal/collector/reddit"
	"github.com/anthropic/autonomous-runner/internal/collector/rss"
)

// buildRegistry creates and populates the collector registry.
func buildRegistry(cfg Config) *collector.Registry {
	reg := collector.NewRegistry()

	// Core collectors
	reg.Register(hackernews.New("top"))
	reg.Register(hackernews.New("ask"))  // Ask HN discussions
	reg.Register(hackernews.New("show")) // Show HN projects
	reg.Register(buildRSSCollector(cfg.RSSFeeds))
	reg.Register(github.New())

	// Additional collectors
	reg.Register(reddit.New())
	reg.Register(lobsters.New())

	return reg
}

// mergeRSSSources returns default RSS sources with optional custom feeds appended.
func mergeRSSSources(custom []RSSFeedConfig) []rss.Source {
	sources := make([]rss.Source, 0, len(rss.DefaultSources)+len(custom))
	sources = append(sources, rss.DefaultSources...)
	for _, f := range custom {
		sources = append(sources, rss.Source{Name: f.Name, URL: f.URL})
	}
	return sources
}

// buildRSSCollector creates RSS collector with optional custom feeds appended to defaults.
func buildRSSCollector(custom []RSSFeedConfig) *rss.Collector {
	if len(custom) == 0 {
		return rss.NewWithDefaults()
	}
	return rss.New(mergeRSSSources(custom))
}
