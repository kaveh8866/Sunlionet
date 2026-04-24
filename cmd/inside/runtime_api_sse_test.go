//go:build !daemon

package main

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRuntimeEventsStreamSSE(t *testing.T) {
	store := newRuntimeStore("real")
	ts := httptest.NewServer(runtimeAPIMux(store))
	defer ts.Close()

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/api/events/stream", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		t.Fatalf("GET stream: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}

	go func() {
		time.Sleep(25 * time.Millisecond)
		store.addEvent("PROFILE_SWITCH", "Selected profile reality-1", map[string]interface{}{"selected": "reality-1"})
		store.addEvent("CONNECTION_FAIL", "HTTP probe failed", map[string]interface{}{"profile": "reality-1", "reason": "DNS_FAILURE"})
	}()

	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 1024), 1024*1024)

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
		if !sc.Scan() {
			if err := sc.Err(); err != nil {
				t.Fatalf("scan: %v", err)
			}
			t.Fatalf("stream closed early, got=%v", got)
		}
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
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order mismatch: got=%v want=%v", got, want)
		}
	}
}
