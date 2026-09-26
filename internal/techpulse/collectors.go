package techpulse

import (
	"time"

	"github.com/majiayu000/techpulse/internal/collector"
	"github.com/majiayu000/techpulse/internal/collector/github"
	"github.com/majiayu000/techpulse/internal/collector/hackernews"
	"github.com/majiayu000/techpulse/internal/collector/lobsters"
	"github.com/majiayu000/techpulse/internal/collector/reddit"
	"github.com/majiayu000/techpulse/internal/collector/rss"
	"github.com/majiayu000/techpulse/internal/httpclient"
)

// buildRegistry creates and populates the collector registry. Each collector's
// HTTP client receives the configured per-request timeout so multi-request
// sources are not cut off by a cumulative Collect deadline.
func buildRegistry(cfg Config) *collector.Registry {
	reg := collector.NewRegistry()
	httpOpts := requestHTTPOptions(cfg.Timeout)
	reqTimeout := requestClientTimeout(cfg.Timeout)

	reg.Register(hackernews.New("top", httpOpts...))
	reg.Register(hackernews.New("ask", httpOpts...))
	reg.Register(hackernews.New("show", httpOpts...))
	reg.Register(buildRSSCollector(cfg.RSSFeeds, httpOpts...))
	reg.Register(withHTTPClientTimeout(github.New(), reqTimeout))
	reg.Register(withHTTPClientTimeout(reddit.New(), reqTimeout))
	reg.Register(withHTTPClientTimeout(lobsters.New(), reqTimeout))

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
func buildRSSCollector(custom []RSSFeedConfig, opts ...httpclient.Option) *rss.Collector {
	if len(custom) == 0 {
		return rss.NewWithDefaults(opts...)
	}
	return rss.New(mergeRSSSources(custom), opts...)
}

// requestHTTPOptions returns httpclient options that apply the configured
// per-request timeout when positive.
func requestHTTPOptions(seconds int) []httpclient.Option {
	if d := timeoutDuration(seconds); d > 0 {
		return []httpclient.Option{httpclient.WithTimeout(d)}
	}
	return nil
}

// requestClientTimeout returns the http.Client timeout for collectors that
// use the standard library client directly.
func requestClientTimeout(seconds int) time.Duration {
	if d := timeoutDuration(seconds); d > 0 {
		return d
	}
	return 30 * time.Second
}

// httpClientTimeoutSetter is implemented by collectors that expose a mutable
// standard-library HTTP client timeout.
type httpClientTimeoutSetter interface {
	collector.Collector
	SetHTTPTimeout(d time.Duration)
}

func withHTTPClientTimeout(c httpClientTimeoutSetter, d time.Duration) collector.Collector {
	c.SetHTTPTimeout(d)
	return c
}
