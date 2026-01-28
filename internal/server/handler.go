package server

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"
)

// ServerConfig holds all server configuration.
type ServerConfig struct {
	Port              int
	RefreshIntervalMs int
	SSEPingMs         int
	AutoRefresh       bool
	SiteDir           string
	DataPath          string
	BaseDir           string
	FetchBin          string
	FetchArgs         []string
	PolyBook          PolyBookConfig
}

// Cache holds the server-side data cache.
type Cache struct {
	mu          sync.RWMutex
	jsonData    []byte
	etag        string
	generatedAt string
	count       int
	loadedAt    string
}

// Server is the main HTTP server.
type Server struct {
	cfg       ServerConfig
	cache     *Cache
	broker    *SSEBroker
	polyProxy *PolyBookProxy
	updating  bool
	updMu     sync.Mutex
	lastError error
}

// NewServer creates a new server.
func NewServer(cfg ServerConfig) *Server {
	return &Server{
		cfg:       cfg,
		cache:     &Cache{},
		broker:    NewSSEBroker(cfg.SSEPingMs),
		polyProxy: NewPolyBookProxy(cfg.PolyBook),
	}
}

// LoadCacheFromDisk reads and caches the data file.
func (s *Server) LoadCacheFromDisk() error {
	data, err := os.ReadFile(s.cfg.DataPath)
	if err != nil {
		return err
	}

	hash := sha1.Sum(data)
	etag := hex.EncodeToString(hash[:])

	var parsed struct {
		GeneratedAt string        `json:"generatedAt"`
		Count       int           `json:"count"`
		Pairs       []interface{} `json:"pairs"`
		Markets     []interface{} `json:"markets"`
	}
	json.Unmarshal(data, &parsed)

	count := parsed.Count
	if count == 0 {
		count = len(parsed.Pairs)
		if count == 0 {
			count = len(parsed.Markets)
		}
	}

	s.cache.mu.Lock()
	s.cache.jsonData = data
	s.cache.etag = etag
	s.cache.generatedAt = parsed.GeneratedAt
	s.cache.count = count
	s.cache.loadedAt = time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	s.cache.mu.Unlock()

	return nil
}

// RunFetchScript executes the fetch_all_markets binary.
func (s *Server) RunFetchScript() error {
	cmd := exec.Command(s.cfg.FetchBin, s.cfg.FetchArgs...)
	cmd.Dir = s.cfg.BaseDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// UpdateData triggers a data refresh.
func (s *Server) UpdateData() bool {
	s.updMu.Lock()
	if s.updating {
		s.updMu.Unlock()
		return false
	}
	s.updating = true
	s.updMu.Unlock()

	s.broker.Broadcast("status", map[string]interface{}{
		"state": "updating",
		"at":    time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
	})

	err := s.RunFetchScript()
	if err == nil {
		err = s.LoadCacheFromDisk()
	}

	s.updMu.Lock()
	s.updating = false
	if err != nil {
		s.lastError = err
	} else {
		s.lastError = nil
	}
	s.updMu.Unlock()

	if err != nil {
		s.broker.Broadcast("status", map[string]interface{}{
			"state":   "error",
			"message": err.Error(),
			"at":      time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
		})
		return false
	}

	s.cache.mu.RLock()
	s.broker.Broadcast("update", map[string]interface{}{
		"generatedAt": s.cache.generatedAt,
		"count":       s.cache.count,
		"loadedAt":    s.cache.loadedAt,
	})
	s.cache.mu.RUnlock()
	return true
}

// ShouldRefreshOnStart checks if data is stale.
func (s *Server) ShouldRefreshOnStart() bool {
	info, err := os.Stat(s.cfg.DataPath)
	if err != nil {
		return true
	}
	ageMs := time.Since(info.ModTime()).Milliseconds()
	return ageMs > int64(float64(s.cfg.RefreshIntervalMs)*0.6)
}

// ServeHTTP is the main HTTP handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	switch {
	case path == "/api/markets" && r.Method == "GET":
		s.serveMarkets(w, r)
	case path == "/api/status" && r.Method == "GET":
		s.serveStatus(w)
	case path == "/api/refresh" && r.Method == "POST":
		s.handleRefresh(w)
	case path == "/api/stream" && r.Method == "GET":
		s.handleStream(w, r)
	case path == "/api/polymarket/orderbook" && r.Method == "GET":
		s.polyProxy.HandleOrderbook(w, r)
	default:
		ServeStatic(w, path, s.cfg.SiteDir)
	}
}

func (s *Server) serveMarkets(w http.ResponseWriter, r *http.Request) {
	s.cache.mu.RLock()
	data := s.cache.jsonData
	etag := s.cache.etag
	s.cache.mu.RUnlock()

	if data == nil {
		if err := s.LoadCacheFromDisk(); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(503)
			json.NewEncoder(w).Encode(map[string]string{"error": "Market data not ready yet."})
			return
		}
		s.cache.mu.RLock()
		data = s.cache.jsonData
		etag = s.cache.etag
		s.cache.mu.RUnlock()
	}

	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(304)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("ETag", etag)
	w.Write(data)
}

func (s *Server) serveStatus(w http.ResponseWriter) {
	s.cache.mu.RLock()
	generatedAt := s.cache.generatedAt
	count := s.cache.count
	loadedAt := s.cache.loadedAt
	s.cache.mu.RUnlock()

	s.updMu.Lock()
	updating := s.updating
	var errMsg *string
	if s.lastError != nil {
		e := s.lastError.Error()
		errMsg = &e
	}
	s.updMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"generatedAt": generatedAt,
		"count":       count,
		"updating":    updating,
		"loadedAt":    loadedAt,
		"lastError":   errMsg,
	})
}

func (s *Server) handleRefresh(w http.ResponseWriter) {
	s.updMu.Lock()
	busy := s.updating
	s.updMu.Unlock()

	if busy {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(409)
		json.NewEncoder(w).Encode(map[string]string{"status": "busy"})
		return
	}

	go s.UpdateData()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(202)
	json.NewEncoder(w).Encode(map[string]string{"status": "started"})
}

func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	s.cache.mu.RLock()
	hello := map[string]interface{}{
		"generatedAt": s.cache.generatedAt,
		"count":       s.cache.count,
		"loadedAt":    s.cache.loadedAt,
	}
	s.cache.mu.RUnlock()

	s.broker.HandleStream(w, r, hello)
}

// Start starts the server.
func (s *Server) Start() error {
	// Try loading existing cache
	s.LoadCacheFromDisk()

	if s.cfg.AutoRefresh {
		if s.ShouldRefreshOnStart() {
			go s.UpdateData()
		}
		go func() {
			ticker := time.NewTicker(time.Duration(s.cfg.RefreshIntervalMs) * time.Millisecond)
			for range ticker.C {
				s.UpdateData()
			}
		}()
	}

	addr := fmt.Sprintf(":%d", s.cfg.Port)
	fmt.Printf("Server listening on http://localhost:%d (refresh every %dms)\n",
		s.cfg.Port, s.cfg.RefreshIntervalMs)
	return http.ListenAndServe(addr, s)
}
