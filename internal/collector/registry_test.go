package collector

import (
	"context"
	"errors"
	"testing"
	"time"
)

// mockCollector is a test implementation of Collector.
type mockCollector struct {
	name      string
	articles  []Article
	err       error
	validated bool
	validErr  error
}

func (m *mockCollector) Name() string {
	return m.name
}

func (m *mockCollector) Collect(ctx context.Context, opts Options) ([]Article, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.articles, nil
}

func (m *mockCollector) Validate() error {
	m.validated = true
	return m.validErr
}

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("NewRegistry returned nil")
	}
	if r.collectors == nil {
		t.Error("collectors map is nil")
	}
}

func TestRegistryRegister(t *testing.T) {
	r := NewRegistry()
	c := &mockCollector{name: "test"}

	err := r.Register(c)
	if err != nil {
		t.Errorf("Register failed: %v", err)
	}
	if !c.validated {
		t.Error("Validate was not called")
	}

	got, ok := r.Get("test")
	if !ok {
		t.Error("Get returned false for registered collector")
	}
	if got.Name() != "test" {
		t.Errorf("expected name 'test', got '%s'", got.Name())
	}
}

func TestRegistryRegisterValidationError(t *testing.T) {
	r := NewRegistry()
	c := &mockCollector{
		name:     "invalid",
		validErr: errors.New("validation failed"),
	}

	err := r.Register(c)
	if err == nil {
		t.Error("Register should fail with validation error")
	}

	_, ok := r.Get("invalid")
	if ok {
		t.Error("invalid collector should not be registered")
	}
}

func TestRegistryGet(t *testing.T) {
	r := NewRegistry()

	// Non-existent collector
	_, ok := r.Get("nonexistent")
	if ok {
		t.Error("Get should return false for non-existent collector")
	}

	// Register and get
	r.Register(&mockCollector{name: "exists"})
	c, ok := r.Get("exists")
	if !ok {
		t.Error("Get should return true for existing collector")
	}
	if c.Name() != "exists" {
		t.Errorf("expected name 'exists', got '%s'", c.Name())
	}
}

func TestRegistryAll(t *testing.T) {
	r := NewRegistry()

	// Empty registry
	all := r.All()
	if len(all) != 0 {
		t.Errorf("expected 0 collectors, got %d", len(all))
	}

	// With collectors — register out of name order to prove sorting.
	r.Register(&mockCollector{name: "c3"})
	r.Register(&mockCollector{name: "c1"})
	r.Register(&mockCollector{name: "c2"})

	all = r.All()
	if len(all) != 3 {
		t.Errorf("expected 3 collectors, got %d", len(all))
	}
	want := []string{"c1", "c2", "c3"}
	for i, c := range all {
		if c.Name() != want[i] {
			t.Errorf("All()[%d] = %q, want %q (stable name order)", i, c.Name(), want[i])
		}
	}
}

func TestRegistryNames(t *testing.T) {
	r := NewRegistry()

	r.Register(&mockCollector{name: "beta"})
	r.Register(&mockCollector{name: "alpha"})

	names := r.Names()
	if len(names) != 2 {
		t.Errorf("expected 2 names, got %d", len(names))
	}
	if names[0] != "alpha" || names[1] != "beta" {
		t.Errorf("expected sorted names [alpha beta], got %v", names)
	}
}

func TestRegistryCollectAll(t *testing.T) {
	r := NewRegistry()

	c1 := &mockCollector{
		name: "c1",
		articles: []Article{
			{ID: "1", Title: "Article 1"},
		},
	}
	c2 := &mockCollector{
		name: "c2",
		articles: []Article{
			{ID: "2", Title: "Article 2"},
			{ID: "3", Title: "Article 3"},
		},
	}

	r.Register(c1)
	r.Register(c2)

	ctx := context.Background()
	results := r.CollectAll(ctx, DefaultOptions())

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	totalArticles := 0
	for _, res := range results {
		if res.Error != nil {
			t.Errorf("unexpected error for %s: %v", res.Source, res.Error)
		}
		totalArticles += len(res.Articles)
		// NOTE: Duration intentionally not asserted — a mock collector returns
		// instantly, so time.Since can legitimately measure 0.
		if res.Timestamp.IsZero() {
			t.Errorf("expected non-zero timestamp for %s", res.Source)
		}
	}

	if totalArticles != 3 {
		t.Errorf("expected 3 total articles, got %d", totalArticles)
	}
}

func TestRegistryCollectAllWithError(t *testing.T) {
	r := NewRegistry()

	r.Register(&mockCollector{
		name:     "ok",
		articles: []Article{{ID: "1"}},
	})
	r.Register(&mockCollector{
		name: "failing",
		err:  errors.New("network error"),
	})

	ctx := context.Background()
	results := r.CollectAll(ctx, DefaultOptions())

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	var okCount, errCount int
	for _, res := range results {
		if res.Error != nil {
			errCount++
		} else {
			okCount++
		}
	}

	if okCount != 1 || errCount != 1 {
		t.Errorf("expected 1 ok and 1 error, got %d ok and %d errors", okCount, errCount)
	}
}

func TestRegistryCollectAllEmpty(t *testing.T) {
	r := NewRegistry()

	ctx := context.Background()
	results := r.CollectAll(ctx, DefaultOptions())

	if len(results) != 0 {
		t.Errorf("expected 0 results for empty registry, got %d", len(results))
	}
}

func TestRegistryCollectAllContext(t *testing.T) {
	r := NewRegistry()

	// Collector that respects context
	slowCollector := &mockCollector{
		name:     "slow",
		articles: []Article{{ID: "1"}},
	}
	r.Register(slowCollector)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	results := r.CollectAll(ctx, DefaultOptions())

	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}
