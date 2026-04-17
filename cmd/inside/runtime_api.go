package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

type RuntimeSnapshot struct {
	Status        string           `json:"status"`
	ActiveProfile string           `json:"activeProfile"`
	LatencyMs     int64            `json:"latencyMs"`
	LastUpdated   int64            `json:"lastUpdated"`
	Failures      []RuntimeFailure `json:"failures"`
	Mode          string           `json:"mode"`
}

type RuntimeFailure struct {
	Timestamp int64  `json:"timestamp"`
	Reason    string `json:"reason"`
}

type RuntimeEvent struct {
	Timestamp int64                  `json:"timestamp"`
	Type      string                 `json:"type"`
	Message   string                 `json:"message"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

type eventBus struct {
	mu        sync.Mutex
	ring      []RuntimeEvent
	maxRing   int
	subs      map[chan RuntimeEvent]struct{}
	subBufLen int
}

func newEventBus(maxRing int, subBufLen int) *eventBus {
	if maxRing < 1 {
		maxRing = 1
	}
	if subBufLen < 1 {
		subBufLen = 1
	}
	return &eventBus{
		ring:      make([]RuntimeEvent, 0, maxRing),
		maxRing:   maxRing,
		subs:      make(map[chan RuntimeEvent]struct{}),
		subBufLen: subBufLen,
	}
}

func (b *eventBus) publish(ev RuntimeEvent) {
	b.mu.Lock()
	b.ring = append(b.ring, ev)
	if len(b.ring) > b.maxRing {
		b.ring = append([]RuntimeEvent(nil), b.ring[len(b.ring)-b.maxRing:]...)
	}
	for ch := range b.subs {
		sendDropOldest(ch, ev)
	}
	b.mu.Unlock()
}

func (b *eventBus) recent(limit int) []RuntimeEvent {
	b.mu.Lock()
	defer b.mu.Unlock()
	if limit <= 0 || limit > len(b.ring) {
		limit = len(b.ring)
	}
	out := make([]RuntimeEvent, 0, limit)
	out = append(out, b.ring[len(b.ring)-limit:]...)
	return out
}

func (b *eventBus) subscribe() (ch <-chan RuntimeEvent, cancel func()) {
	c := make(chan RuntimeEvent, b.subBufLen)
	b.mu.Lock()
	b.subs[c] = struct{}{}
	b.mu.Unlock()

	return c, func() {
		b.mu.Lock()
		delete(b.subs, c)
		b.mu.Unlock()
		close(c)
	}
}

func sendDropOldest(ch chan RuntimeEvent, ev RuntimeEvent) {
	select {
	case ch <- ev:
		return
	default:
	}
	select {
	case <-ch:
	default:
	}
	select {
	case ch <- ev:
	default:
	}
}

type runtimeStore struct {
	mu       sync.Mutex
	snapshot RuntimeSnapshot
	events   *eventBus
}

func newRuntimeStore(mode string) *runtimeStore {
	now := time.Now().Unix()
	return &runtimeStore{
		snapshot: RuntimeSnapshot{
			Status:      "disconnected",
			LatencyMs:   0,
			LastUpdated: now,
			Failures:    nil,
			Mode:        strings.TrimSpace(mode),
		},
		events: newEventBus(100, 100),
	}
}

func (s *runtimeStore) setStatus(status string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot.Status = status
	s.snapshot.LastUpdated = time.Now().Unix()
}

func (s *runtimeStore) setActiveProfile(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot.ActiveProfile = id
	s.snapshot.LastUpdated = time.Now().Unix()
}

func (s *runtimeStore) setLatencyMs(ms int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot.LatencyMs = ms
	s.snapshot.LastUpdated = time.Now().Unix()
}

func (s *runtimeStore) addFailure(reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot.Failures = append(s.snapshot.Failures, RuntimeFailure{
		Timestamp: time.Now().Unix(),
		Reason:    reason,
	})
	s.snapshot.LastUpdated = time.Now().Unix()
}

func (s *runtimeStore) addEvent(typ string, msg string, meta map[string]interface{}) {
	safeMsg := sanitizeRuntimeText(msg)
	safeMeta := sanitizeRuntimeMeta(meta)
	ev := RuntimeEvent{
		Timestamp: time.Now().Unix(),
		Type:      strings.TrimSpace(typ),
		Message:   safeMsg,
		Metadata:  safeMeta,
	}
	s.events.publish(ev)
	s.mu.Lock()
	s.snapshot.LastUpdated = time.Now().Unix()
	s.mu.Unlock()
	_, _ = json.Marshal(ev)
}

var (
	ipv4Pattern = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
	keyPattern  = regexp.MustCompile(`(?i)(age-secret-key-[a-z0-9]+|[a-f0-9]{64})`)
)

func sanitizeRuntimeMeta(meta map[string]interface{}) map[string]interface{} {
	if len(meta) == 0 {
		return map[string]interface{}{}
	}
	out := make(map[string]interface{}, len(meta))
	for k, v := range meta {
		lk := strings.ToLower(strings.TrimSpace(k))
		switch lk {
		case "config", "config_path", "endpoint", "bundle", "bundle_path", "ip", "url":
			out[k] = "[redacted]"
			continue
		}
		switch t := v.(type) {
		case string:
			out[k] = sanitizeRuntimeText(t)
		default:
			out[k] = v
		}
	}
	return out
}

func sanitizeRuntimeText(in string) string {
	s := strings.TrimSpace(in)
	if s == "" {
		return s
	}
	s = ipv4Pattern.ReplaceAllString(s, "x.x.x.x")
	s = keyPattern.ReplaceAllString(s, "[redacted]")
	if len(s) > 240 {
		s = s[:240] + "…"
	}
	return s
}

func (s *runtimeStore) snapshotJSON() ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return json.Marshal(s.snapshot)
}

func (s *runtimeStore) eventsJSON() ([]byte, error) {
	return json.Marshal(s.events.recent(0))
}

func startRuntimeAPIServer(ctx context.Context, addr string, store *runtimeStore) (*http.Server, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return nil, fmt.Errorf("missing runtime api addr")
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("invalid runtime api addr %q (expected host:port): %w", addr, err)
	}
	host = strings.TrimSpace(host)
	if host != "127.0.0.1" && host != "localhost" {
		return nil, fmt.Errorf("runtime api must bind to localhost only, got host=%q", host)
	}

	mux := http.NewServeMux()

	writeJSON := func(w http.ResponseWriter, body []byte) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(body)
	}

	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, []byte(`{"status":"ok"}`))
	})

	mux.HandleFunc("/api/state", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		raw, err := store.snapshotJSON()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		writeJSON(w, raw)
	})

	mux.HandleFunc("/api/events", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		raw, err := store.eventsJSON()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		writeJSON(w, raw)
	})

	mux.HandleFunc("/api/events/stream", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}

		ch, cancel := store.events.subscribe()
		defer cancel()

		backlog := store.events.recent(100)
		for _, ev := range backlog {
			raw, err := json.Marshal(ev)
			if err != nil {
				continue
			}
			_, _ = fmt.Fprintf(w, "data: %s\n\n", raw)
		}
		flusher.Flush()

		ticker := time.NewTicker(20 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-r.Context().Done():
				return
			case ev, ok := <-ch:
				if !ok {
					return
				}
				raw, err := json.Marshal(ev)
				if err != nil {
					continue
				}
				_, _ = fmt.Fprintf(w, "data: %s\n\n", raw)
				flusher.Flush()
			case <-ticker.C:
				_, _ = fmt.Fprintf(w, ": ping\n\n")
				flusher.Flush()
			}
		}
	})

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 2 * time.Second,
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	srv.Addr = ln.Addr().String()
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_ = srv.Shutdown(shutCtx)
		cancel()
	}()
	go func() { _ = srv.Serve(ln) }()

	return srv, nil
}
