package techpulse

import (
	"context"
	"fmt"
	"time"

	"github.com/majiayu000/techpulse/internal/collector"
	"github.com/majiayu000/techpulse/internal/collector/github"
	"github.com/majiayu000/techpulse/internal/collector/hackernews"
	"github.com/majiayu000/techpulse/internal/collector/lobsters"
	"github.com/majiayu000/techpulse/internal/collector/reddit"
	"github.com/majiayu000/techpulse/internal/collector/rss"
)

// buildRegistry creates and populates the collector registry. Every collector
// is wrapped so collector.Options.Timeout bounds each Collect call, even when
// the underlying collector does not apply Options.Timeout to its own clients.
func buildRegistry(cfg Config) *collector.Registry {
	reg := collector.NewRegistry()

	register := func(c collector.Collector) {
		reg.Register(withCollectTimeout(c, timeoutDuration(cfg.Timeout)))
	}

	// Core collectors
	register(hackernews.New("top"))
	register(hackernews.New("ask"))  // Ask HN discussions
	register(hackernews.New("show")) // Show HN projects
	register(buildRSSCollector(cfg.RSSFeeds))
	register(github.New())

	// Additional collectors
	register(reddit.New())
	register(lobsters.New())

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

// withCollectTimeout wraps a collector so its Collect calls enforce a timeout
// via the context. opts.Timeout wins; fallback (the configured Timeout) is
// used when the caller leaves opts.Timeout unset. Zero disables enforcement.
func withCollectTimeout(inner collector.Collector, fallback time.Duration) collector.Collector {
	return &timeoutCollector{inner: inner, fallback: fallback}
}

// timeoutCollector enforces a deadline on every Collect call.
type timeoutCollector struct {
	inner    collector.Collector
	fallback time.Duration
}

func (t *timeoutCollector) Name() string    { return t.inner.Name() }
func (t *timeoutCollector) Validate() error { return t.inner.Validate() }

// Collect derives a context deadline from Options.Timeout (falling back to
// the configured default) so slow sources fail clearly instead of hanging.
func (t *timeoutCollector) Collect(ctx context.Context, opts collector.Options) ([]collector.Article, error) {
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = t.fallback
	}
	if timeout <= 0 {
		return t.inner.Collect(ctx, opts)
	}

	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	articles, err := t.inner.Collect(cctx, opts)
	// Only relabel as a per-source timeout when the PARENT context is still
	// healthy; otherwise cctx merely inherited the parent's deadline error
	// and the real cause would be misreported for every source.
	if err != nil && ctx.Err() == nil && cctx.Err() == context.DeadlineExceeded {
		return articles, fmt.Errorf("%s exceeded timeout %s: %w", t.inner.Name(), timeout, err)
	}
	return articles, err
}
