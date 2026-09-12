package hackernews

import (
	"context"
	"fmt"
	"sync"

	"github.com/majiayu000/techpulse/internal/logger"
)

const (
	// DefaultConcurrency is the default number of concurrent fetches.
	DefaultConcurrency = 10
)

// itemResult holds the result of fetching an item.
type itemResult struct {
	item *Item
	err  error
}

// fetchResult bundles the successfully fetched items with diagnostics for
// the ones that could not be retrieved.
type fetchResult struct {
	// items are the fetched items, in the order of the requested IDs.
	items []*Item
	// dropped is the number of requested items that were not fetched.
	dropped int
	// err is non-nil when the context was canceled or every single item
	// fetch failed. Partial failures are reported via dropped and a log
	// warning instead of an error.
	err error
}

// FetchItemsConcurrently fetches multiple items concurrently and returns the
// successfully fetched items in their original order. Failures are logged but
// not returned; use fetchItemsConcurrently when the error matters.
func (c *Client) FetchItemsConcurrently(ctx context.Context, ids []int, concurrency int) []*Item {
	return c.fetchItemsConcurrently(ctx, ids, concurrency).items
}

// fetchItemsConcurrently fetches multiple items concurrently, preserving the
// order of ids. Failed fetches are never fabricated: they are counted in
// result.dropped and logged, and result.err is non-nil when the context was
// canceled or every single fetch failed.
func (c *Client) fetchItemsConcurrently(ctx context.Context, ids []int, concurrency int) fetchResult {
	if len(ids) == 0 {
		return fetchResult{}
	}
	if concurrency <= 0 {
		concurrency = DefaultConcurrency
	}

	// Create channels
	idChan := make(chan int, len(ids))
	resultChan := make(chan itemResult, len(ids))

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.fetchWorker(ctx, idChan, resultChan)
		}()
	}

	// Send IDs to workers
	for _, id := range ids {
		idChan <- id
	}
	close(idChan)

	// Wait for all workers to finish
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results in order, tracking failures instead of discarding them.
	resultMap := make(map[int]*Item, len(ids))
	var firstErr error
	for result := range resultChan {
		if result.err == nil && result.item != nil {
			resultMap[result.item.ID] = result.item
			continue
		}
		if firstErr == nil {
			firstErr = result.err
		}
	}

	items := make([]*Item, 0, len(ids))
	for _, id := range ids {
		if item, ok := resultMap[id]; ok {
			items = append(items, item)
		}
	}

	res := fetchResult{items: items, dropped: len(ids) - len(items)}

	// Order matters: a complete fetch must survive a late ctx cancellation,
	// and an empty id list must not read as "all fetches failed".
	switch {
	case len(ids) > 0 && res.dropped == len(ids):
		if firstErr != nil {
			res.err = fmt.Errorf("all %d item fetches failed (first error: %w)", len(ids), firstErr)
		} else {
			res.err = fmt.Errorf("all %d item fetches failed", len(ids))
		}
	case ctx.Err() != nil:
		res.err = fmt.Errorf("fetch interrupted after %d/%d items: %w", len(items), len(ids), ctx.Err())
	case res.dropped > 0:
		logger.Warn("hackernews: some items were dropped during concurrent fetch",
			logger.F("dropped", res.dropped),
			logger.F("total", len(ids)),
			logger.F("example_error", firstErr))
	}

	return res
}

// fetchWorker is a worker goroutine that fetches items.
func (c *Client) fetchWorker(ctx context.Context, ids <-chan int, results chan<- itemResult) {
	for id := range ids {
		select {
		case <-ctx.Done():
			results <- itemResult{err: ctx.Err()}
			return
		default:
			item, err := c.FetchItem(ctx, id)
			results <- itemResult{item: item, err: err}
		}
	}
}
