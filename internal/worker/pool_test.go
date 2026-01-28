package worker

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
)

func TestRunOrder(t *testing.T) {
	items := []int{1, 2, 3, 4, 5}
	results, errs := Run(context.Background(), items, 2, func(ctx context.Context, item int) (int, error) {
		return item * 10, nil
	})

	for i, r := range results {
		want := items[i] * 10
		if r != want {
			t.Errorf("results[%d] = %d, want %d", i, r, want)
		}
		if errs[i] != nil {
			t.Errorf("errs[%d] = %v, want nil", i, errs[i])
		}
	}
}

func TestRunErrors(t *testing.T) {
	items := []int{1, 2, 3}
	_, errs := Run(context.Background(), items, 2, func(ctx context.Context, item int) (int, error) {
		if item == 2 {
			return 0, fmt.Errorf("error on %d", item)
		}
		return item, nil
	})

	if errs[0] != nil {
		t.Errorf("errs[0] = %v, want nil", errs[0])
	}
	if errs[1] == nil {
		t.Error("errs[1] = nil, want error")
	}
	if errs[2] != nil {
		t.Errorf("errs[2] = %v, want nil", errs[2])
	}
}

func TestRunConcurrencyLimit(t *testing.T) {
	var maxConcurrent int64
	var current int64

	items := make([]int, 20)
	for i := range items {
		items[i] = i
	}

	Run(context.Background(), items, 3, func(ctx context.Context, item int) (int, error) {
		c := atomic.AddInt64(&current, 1)
		for {
			old := atomic.LoadInt64(&maxConcurrent)
			if c <= old {
				break
			}
			if atomic.CompareAndSwapInt64(&maxConcurrent, old, c) {
				break
			}
		}
		atomic.AddInt64(&current, -1)
		return item, nil
	})

	mc := atomic.LoadInt64(&maxConcurrent)
	if mc > 3 {
		t.Errorf("max concurrent = %d, want <= 3", mc)
	}
}

func TestRunEmpty(t *testing.T) {
	results, errs := Run(context.Background(), []int{}, 2, func(ctx context.Context, item int) (int, error) {
		return item, nil
	})

	if len(results) != 0 {
		t.Errorf("results len = %d, want 0", len(results))
	}
	if len(errs) != 0 {
		t.Errorf("errs len = %d, want 0", len(errs))
	}
}

func TestRunCollect(t *testing.T) {
	items := []int{1, 2, 3}
	results := RunCollect(context.Background(), items, 2, func(ctx context.Context, item int) (int, error) {
		return item * 2, nil
	})

	for i, r := range results {
		want := items[i] * 2
		if r != want {
			t.Errorf("results[%d] = %d, want %d", i, r, want)
		}
	}
}
