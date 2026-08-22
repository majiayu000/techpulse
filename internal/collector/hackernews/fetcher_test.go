package hackernews

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
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

	client := newFastTestClient(server.URL)
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

	client := newFastTestClient(server.URL)
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

	client := newFastTestClient(server.URL)
	ids := []int{1, 2, 3, 4, 5, 6}

	res := client.fetchItemsConcurrently(context.Background(), ids, 1)
	items := res.items

	// Should have some items (not all, due to errors)
	if len(items) == 0 {
		t.Error("expected some items even with errors")
	}
	if len(items) >= 6 {
		t.Error("expected some failures")
	}
	// Partial failure is reported as a dropped count, not an error.
	if res.err != nil {
		t.Errorf("expected no error on partial failure, got %v", res.err)
	}
	if res.dropped != 6-len(items) {
		t.Errorf("expected dropped=%d, got %d", 6-len(items), res.dropped)
	}
}

func TestFetchItemsConcurrently_AllFailed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "error", http.StatusInternalServerError)
	}))
	defer server.Close()

	client := newFastTestClient(server.URL)

	res := client.fetchItemsConcurrently(context.Background(), []int{1, 2, 3}, 2)

	if len(res.items) != 0 {
		t.Errorf("expected no items, got %d", len(res.items))
	}
	if res.err == nil {
		t.Fatal("expected an error when every item fetch fails")
	}
	if !strings.Contains(res.err.Error(), "all 3 item fetches failed") {
		t.Errorf("error should report the all-failed count, got: %v", res.err)
	}
}

func TestFetchItemsConcurrently_NullResponsesTracked(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := parseItemIDFromPath(r.URL.Path)
		// Firebase answers "null" for unknown or purged items.
		if id == 2 || id == 4 {
			w.Write([]byte("null"))
			return
		}
		item := Item{ID: id, Title: "Test", Time: time.Now().Unix()}
		json.NewEncoder(w).Encode(item)
	}))
	defer server.Close()

	client := newFastTestClient(server.URL)

	res := client.fetchItemsConcurrently(context.Background(), []int{1, 2, 3, 4}, 4)

	// Null responses are failures, not silent zero-value items.
	if res.dropped != 2 {
		t.Errorf("expected 2 dropped (null responses), got %d", res.dropped)
	}
	if res.err != nil {
		t.Errorf("expected no error on partial failure, got %v", res.err)
	}
	gotIDs := []int{}
	for _, item := range res.items {
		gotIDs = append(gotIDs, item.ID)
	}
	if len(gotIDs) != 2 || gotIDs[0] != 1 || gotIDs[1] != 3 {
		t.Errorf("expected surviving items [1 3] in order, got %v", gotIDs)
	}
}

func TestFetchItem_NullResponseBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("null"))
	}))
	defer server.Close()

	client := newFastTestClient(server.URL)

	item, err := client.FetchItem(context.Background(), 7)
	if err == nil {
		t.Fatal("expected an error for a null response body")
	}
	if item != nil {
		t.Errorf("expected no item, got %+v", item)
	}
	if !strings.Contains(err.Error(), "7") {
		t.Errorf("error should mention the requested ID, got: %v", err)
	}
}

func TestFetchItemsConcurrently_DefaultConcurrency(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := parseItemIDFromPath(r.URL.Path)
		item := Item{ID: id, Title: "Test", Time: time.Now().Unix()}
		json.NewEncoder(w).Encode(item)
	}))
	defer server.Close()

	client := newFastTestClient(server.URL)

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

	client := newFastTestClient(server.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	res := client.fetchItemsConcurrently(ctx, []int{1, 2, 3, 4, 5}, 2)

	// Should have fewer items due to cancellation
	if len(res.items) >= 5 {
		t.Error("expected fewer items due to context cancellation")
	}

	// Cancellation must be surfaced, not swallowed.
	if res.err == nil {
		t.Fatal("expected context cancellation to surface an error")
	}
	if !errors.Is(res.err, context.DeadlineExceeded) {
		t.Errorf("error should wrap context.DeadlineExceeded, got: %v", res.err)
	}
}
