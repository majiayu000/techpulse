package hackernews

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestFetchItemsConcurrently(t *testing.T) {
	// Track concurrent requests
	var maxConcurrent int32
	var currentConcurrent int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Track concurrency
		current := atomic.AddInt32(&currentConcurrent, 1)
		defer atomic.AddInt32(&currentConcurrent, -1)

		// Update max
		for {
			max := atomic.LoadInt32(&maxConcurrent)
			if current <= max || atomic.CompareAndSwapInt32(&maxConcurrent, max, current) {
				break
			}
		}

		// Small delay to test concurrency
		time.Sleep(10 * time.Millisecond)

		// Parse ID from URL path
		id := parseItemIDFromPath(r.URL.Path)
		item := Item{ID: id, Title: "Test", Time: time.Now().Unix()}
		json.NewEncoder(w).Encode(item)
	}))
	defer server.Close()

	client := NewClientWithBaseURL(server.URL)
	ids := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	items := client.FetchItemsConcurrently(context.Background(), ids, 5)

	// Should have fetched all items
	if len(items) != 10 {
		t.Errorf("expected 10 items, got %d", len(items))
	}

	// Should have used concurrency
	if maxConcurrent < 2 {
		t.Errorf("expected concurrent requests, got max %d", maxConcurrent)
	}
}

func TestFetchItemsConcurrently_PreservesOrder(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := parseItemIDFromPath(r.URL.Path)
		item := Item{ID: id, Title: "Test", Time: time.Now().Unix()}
		json.NewEncoder(w).Encode(item)
	}))
	defer server.Close()

	client := NewClientWithBaseURL(server.URL)
	ids := []int{5, 3, 1, 4, 2}

	items := client.FetchItemsConcurrently(context.Background(), ids, 2)

	// Should preserve original order
	if len(items) != 5 {
		t.Fatalf("expected 5 items, got %d", len(items))
	}

	for i, expectedID := range ids {
		if items[i].ID != expectedID {
			t.Errorf("item %d: expected ID %d, got %d", i, expectedID, items[i].ID)
		}
	}
}

func TestFetchItemsConcurrently_HandlesErrors(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		// Fail every other request
		if callCount%2 == 0 {
			http.Error(w, "error", http.StatusInternalServerError)
			return
		}
		item := Item{ID: callCount, Title: "Test", Time: time.Now().Unix()}
		json.NewEncoder(w).Encode(item)
	}))
	defer server.Close()

	client := NewClientWithBaseURL(server.URL)
	ids := []int{1, 2, 3, 4, 5, 6}

	items := client.FetchItemsConcurrently(context.Background(), ids, 1)

	// Should have some items (not all, due to errors)
	if len(items) == 0 {
		t.Error("expected some items even with errors")
	}
	if len(items) >= 6 {
		t.Error("expected some failures")
	}
}

func TestFetchItemsConcurrently_DefaultConcurrency(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := parseItemIDFromPath(r.URL.Path)
		item := Item{ID: id, Title: "Test", Time: time.Now().Unix()}
		json.NewEncoder(w).Encode(item)
	}))
	defer server.Close()

	client := NewClientWithBaseURL(server.URL)

	// Test with 0 concurrency (should use default)
	items := client.FetchItemsConcurrently(context.Background(), []int{1, 2, 3}, 0)
	if len(items) != 3 {
		t.Errorf("expected 3 items, got %d", len(items))
	}

	// Test with negative concurrency (should use default)
	items = client.FetchItemsConcurrently(context.Background(), []int{4, 5, 6}, -1)
	if len(items) != 3 {
		t.Errorf("expected 3 items, got %d", len(items))
	}
}

func TestFetchItemsConcurrently_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Slow response
		time.Sleep(100 * time.Millisecond)
		item := Item{ID: 1, Title: "Test", Time: time.Now().Unix()}
		json.NewEncoder(w).Encode(item)
	}))
	defer server.Close()

	client := NewClientWithBaseURL(server.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	items := client.FetchItemsConcurrently(ctx, []int{1, 2, 3, 4, 5}, 2)

	// Should have fewer items due to cancellation
	if len(items) >= 5 {
		t.Error("expected fewer items due to context cancellation")
	}
}
