package server

import (
	"net/http/httptest"
	"sync"
	"testing"
)

// #6: the per-connection ping goroutine and Broadcast both write to the same
// client.w. Without per-client write serialization that is a data race on the
// underlying writer. Run under `go test -race` to catch a regression.
func TestSSEConcurrentWriteIsSerialized(t *testing.T) {
	rec := httptest.NewRecorder() // implements http.Flusher
	client := &SSEClient{w: rec, flusher: rec, done: make(chan struct{})}

	broker := NewSSEBroker(10)
	broker.mu.Lock()
	broker.clients[client] = struct{}{}
	broker.mu.Unlock()

	const iters = 200
	var wg sync.WaitGroup
	wg.Add(2)

	// Goroutine 1: simulates the HandleStream ping loop writing to client.w.
	go func() {
		defer wg.Done()
		for i := 0; i < iters; i++ {
			client.write(": ping\n\n")
		}
	}()

	// Goroutine 2: Broadcast writes to the same client.w concurrently.
	go func() {
		defer wg.Done()
		for i := 0; i < iters; i++ {
			broker.Broadcast("update", map[string]int{"n": i})
		}
	}()

	wg.Wait()

	if rec.Body.Len() == 0 {
		t.Fatal("expected writes to reach the client buffer")
	}
}
