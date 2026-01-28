package worker

import (
	"context"
	"sync"
)

// Run executes fn for each item in items with up to limit concurrent goroutines.
// Results are returned in the same order as items.
// If any task returns an error, it is stored in the result; Run itself does not
// short-circuit (all items are processed). Use RunFailFast for early termination.
func Run[T any, R any](ctx context.Context, items []T, limit int, fn func(context.Context, T) (R, error)) ([]R, []error) {
	if limit < 1 {
		limit = 1
	}
	n := len(items)
	results := make([]R, n)
	errs := make([]error, n)

	var mu sync.Mutex
	idx := 0

	var wg sync.WaitGroup
	for i := 0; i < limit && i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				mu.Lock()
				current := idx
				idx++
				mu.Unlock()

				if current >= n {
					return
				}

				select {
				case <-ctx.Done():
					errs[current] = ctx.Err()
					return
				default:
				}

				r, err := fn(ctx, items[current])
				results[current] = r
				errs[current] = err
			}
		}()
	}

	wg.Wait()
	return results, errs
}

// RunCollect is like Run but returns only the results, ignoring errors.
func RunCollect[T any, R any](ctx context.Context, items []T, limit int, fn func(context.Context, T) (R, error)) []R {
	results, _ := Run(ctx, items, limit, fn)
	return results
}
