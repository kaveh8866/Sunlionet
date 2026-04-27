//go:build !daemon

package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type streamingRecorder struct {
	mu     sync.Mutex
	header http.Header
	code   int
	body   bytes.Buffer
}

func newStreamingRecorder() *streamingRecorder {
	return &streamingRecorder{header: make(http.Header), code: http.StatusOK}
}

func (r *streamingRecorder) Header() http.Header { return r.header }

func (r *streamingRecorder) WriteHeader(code int) { r.code = code }

func (r *streamingRecorder) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.body.Write(p)
}

func (r *streamingRecorder) Flush() {}

func (r *streamingRecorder) BodyString() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.body.String()
}

func TestRuntimeEventsStreamSSE(t *testing.T) {
	store := newRuntimeStore("real")
	h := runtimeAPIMux(store)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "http://example/api/events/stream", nil).WithContext(ctx)
	rec := newStreamingRecorder()
	done := make(chan struct{})

	go func() {
		h.ServeHTTP(rec, req)
		close(done)
	}()

	go func() {
		time.Sleep(25 * time.Millisecond)
		store.addEvent("PROFILE_SWITCH", "Selected profile reality-1", map[string]interface{}{"selected": "reality-1"})
		store.addEvent("CONNECTION_FAIL", "HTTP probe failed", map[string]interface{}{"profile": "reality-1", "reason": "DNS_FAILURE"})
	}()

	want := []string{"PROFILE_SWITCH", "CONNECTION_FAIL"}
	got := make([]string, 0, len(want))

	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()

	for len(got) < len(want) {
		select {
		case <-deadline.C:
			t.Fatalf("timeout waiting for events, got=%v", got)
		default:
		}

		body := rec.BodyString()
		sc := bufio.NewScanner(strings.NewReader(body))
		sc.Buffer(make([]byte, 0, 1024), 1024*1024)
		got = got[:0]
		for sc.Scan() {
			line := sc.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			raw := strings.TrimPrefix(line, "data: ")
			var ev RuntimeEvent
			if err := json.Unmarshal([]byte(raw), &ev); err != nil {
				t.Fatalf("unmarshal event: %v raw=%q", err, raw)
			}
			got = append(got, ev.Type)
			if len(got) >= len(want) {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}

	cancel()
	<-done

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order mismatch: got=%v want=%v", got, want)
		}
	}
}
