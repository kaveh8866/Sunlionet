package mobile

import (
	"encoding/json"
	"runtime"
	"testing"
)

func TestGetStatusBytesReturnsJSON(t *testing.T) {
	raw := GetStatusBytes()
	if len(raw) == 0 {
		t.Fatal("empty status")
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("status is not json: %v", err)
	}
}

func TestRuntimeMemoryStatsJSON(t *testing.T) {
	ConfigureAndroidRuntime(50, 1, 64*1024*1024)
	raw := RuntimeMemoryStatsJSON()
	var decoded map[string]any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatalf("stats is not json: %v", err)
	}
	if runtime.GOMAXPROCS(0) < 1 {
		t.Fatal("invalid gomaxprocs")
	}
}

func TestStartLocalPprofRejectsNonLoopback(t *testing.T) {
	if err := StartLocalPprof("0.0.0.0:0"); err == nil {
		t.Fatal("expected non-loopback pprof rejection")
	}
}
