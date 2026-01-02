package hackernews

import (
	"context"
	"sync"
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

// FetchItemsConcurrently fetches multiple items concurrently.
func (c *Client) FetchItemsConcurrently(ctx context.Context, ids []int, concurrency int) []*Item {
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

	// Collect results in order
	return c.collectResults(ids, resultChan)
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

// collectResults collects results and returns items in the original order.
func (c *Client) collectResults(ids []int, results <-chan itemResult) []*Item {
	// Create a map to store results by ID
	resultMap := make(map[int]*Item)

	// Collect all results
	for result := range results {
		if result.err == nil && result.item != nil {
			resultMap[result.item.ID] = result.item
		}
	}

	// Return items in original order
	items := make([]*Item, 0, len(ids))
	for _, id := range ids {
		if item, ok := resultMap[id]; ok {
			items = append(items, item)
		}
	}

	return items
}
