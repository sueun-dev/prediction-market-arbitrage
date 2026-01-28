package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// SSEBroker manages Server-Sent Events clients.
type SSEBroker struct {
	mu      sync.RWMutex
	clients map[*SSEClient]struct{}
	pingMs  int
}

// SSEClient is a connected SSE client.
type SSEClient struct {
	w       http.ResponseWriter
	flusher http.Flusher
	done    chan struct{}
}

// NewSSEBroker creates a new SSE broker.
func NewSSEBroker(pingMs int) *SSEBroker {
	return &SSEBroker{
		clients: make(map[*SSEClient]struct{}),
		pingMs:  pingMs,
	}
}

// HandleStream serves an SSE connection.
func (b *SSEBroker) HandleStream(w http.ResponseWriter, r *http.Request, hello interface{}) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(200)

	client := &SSEClient{w: w, flusher: flusher, done: make(chan struct{})}

	b.mu.Lock()
	b.clients[client] = struct{}{}
	b.mu.Unlock()

	// Send hello event
	helloData, _ := json.Marshal(hello)
	fmt.Fprintf(w, "event: hello\ndata: %s\n\n", helloData)
	flusher.Flush()

	// Ping timer
	ticker := time.NewTicker(time.Duration(b.pingMs) * time.Millisecond)
	defer ticker.Stop()

	notify := r.Context().Done()

	for {
		select {
		case <-notify:
			b.mu.Lock()
			delete(b.clients, client)
			b.mu.Unlock()
			close(client.done)
			return
		case <-ticker.C:
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}

// Broadcast sends an event to all connected clients.
func (b *SSEBroker) Broadcast(event string, payload interface{}) {
	data, _ := json.Marshal(payload)
	msg := fmt.Sprintf("event: %s\ndata: %s\n\n", event, data)

	b.mu.RLock()
	defer b.mu.RUnlock()

	for client := range b.clients {
		fmt.Fprint(client.w, msg)
		client.flusher.Flush()
	}
}
