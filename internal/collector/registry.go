package collector

import (
	"context"
	"sort"
	"sync"
	"time"
)

// Registry manages registered collectors.
type Registry struct {
	mu         sync.RWMutex
	collectors map[string]Collector
}

// NewRegistry creates a new collector registry.
func NewRegistry() *Registry {
	return &Registry{
		collectors: make(map[string]Collector),
	}
}

// Register adds a collector to the registry.
func (r *Registry) Register(c Collector) error {
	if err := c.Validate(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.collectors[c.Name()] = c
	return nil
}

// Get retrieves a collector by name.
func (r *Registry) Get(name string) (Collector, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.collectors[name]
	return c, ok
}

// All returns all registered collectors in stable name order.
// Sorting avoids map-iteration nondeterminism so downstream fuzzy
// dedup (which keeps the first-seen survivor) retains the same URL
// and source across identical runs.
func (r *Registry) All() []Collector {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Collector, 0, len(r.collectors))
	for _, c := range r.collectors {
		result = append(result, c)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name() < result[j].Name()
	})
	return result
}

// CollectAll runs all collectors concurrently and returns results.
func (r *Registry) CollectAll(ctx context.Context, opts Options) []Result {
	collectors := r.All()
	results := make([]Result, len(collectors))

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

// Names returns all registered collector names in stable sorted order.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.collectors))
	for name := range r.collectors {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ProgressCallback is called when a collector starts or finishes.
type ProgressCallback struct {
	OnStart    func(name string)
	OnComplete func(name string, count int)
	OnError    func(name string, err error)
}

// CollectAllWithProgress runs collectors concurrently with progress callbacks.
func (r *Registry) CollectAllWithProgress(ctx context.Context, opts Options, cb ProgressCallback) []Result {
	collectors := r.All()
	results := make([]Result, len(collectors))

	var wg sync.WaitGroup
	for i, c := range collectors {
		wg.Add(1)
		go func(idx int, col Collector) {
			defer wg.Done()
			name := col.Name()
			if cb.OnStart != nil {
				cb.OnStart(name)
			}
			start := time.Now()
			articles, err := col.Collect(ctx, opts)
			results[idx] = Result{
				Source:    name,
				Articles:  articles,
				Error:     err,
				Duration:  time.Since(start),
				Timestamp: time.Now(),
			}
			if err != nil && cb.OnError != nil {
				cb.OnError(name, err)
			} else if cb.OnComplete != nil {
				cb.OnComplete(name, len(articles))
			}
		}(i, c)
	}
	wg.Wait()
	return results
}
