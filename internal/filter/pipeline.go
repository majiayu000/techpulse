package filter

import (
	"github.com/anthropic/autonomous-runner/internal/collector"
)

// Pipeline chains multiple filters together.
type Pipeline struct {
	filters []Filter
	dedup   *DedupFilter
}

// NewPipeline creates a new filter pipeline.
func NewPipeline(filters ...Filter) *Pipeline {
	return &Pipeline{filters: filters}
}

// Add adds a filter to the pipeline.
func (p *Pipeline) Add(f Filter) *Pipeline {
	p.filters = append(p.filters, f)
	return p
}

// WithDedup adds a deduplication filter to the pipeline.
func (p *Pipeline) WithDedup(similarity float64) *Pipeline {
	p.dedup = NewDedupFilter(similarity)
	return p
}

// Process runs all filters on the articles.
func (p *Pipeline) Process(articles []collector.Article) []FilteredArticle {
	if len(p.filters) == 0 && p.dedup == nil {
		return ToFiltered(articles)
	}

	var result []FilteredArticle

	// Apply first filter
	if len(p.filters) > 0 {
		result = p.filters[0].Apply(articles)

		// Apply remaining filters
		for i := 1; i < len(p.filters); i++ {
			result = p.filters[i].Apply(ToArticles(result))
		}
	} else {
		result = ToFiltered(articles)
	}

	// Apply deduplication last
	if p.dedup != nil {
		result = p.dedup.ApplyToFiltered(result)
	}

	return result
}

// Reset resets all stateful filters.
func (p *Pipeline) Reset() {
	if p.dedup != nil {
		p.dedup.Reset()
	}
}

// DefaultPipeline creates a pipeline with default filters.
func DefaultPipeline() *Pipeline {
	include, exclude := DefaultKeywords()
	return NewPipeline(
		NewKeywordFilter(include, exclude),
	).WithDedup(0.8)
}
