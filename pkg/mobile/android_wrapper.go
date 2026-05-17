package mobile

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof"
	"runtime"
	"runtime/debug"
	"sync"

	"github.com/kaveh/sunlionet-agent/pkg/mobilebridge"
)

var androidRuntimeOnce sync.Once

func ConfigureAndroidRuntime(gcPercent int, maxProcs int, memoryLimitBytes int64) {
	androidRuntimeOnce.Do(func() {
		if gcPercent <= 0 {
			gcPercent = 75
		}
		if maxProcs <= 0 {
			maxProcs = 2
		}
		if maxProcs > runtime.NumCPU() {
			maxProcs = runtime.NumCPU()
		}
		debug.SetGCPercent(gcPercent)
		runtime.GOMAXPROCS(maxProcs)
		if memoryLimitBytes > 0 {
			debug.SetMemoryLimit(memoryLimitBytes)
		}
	})
}

func StartAgentBytes(config []byte) error {
	var cfg mobilebridge.AgentConfig
	if err := json.Unmarshal(config, &cfg); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}
	if err := validateConfig(&cfg); err != nil {
		return err
	}
	mobilebridge.StartAgent(string(config))
	return nil
}

func ImportOnboardingURIWithConfigBytes(uri []byte, config []byte) error {
	return mobilebridge.ImportOnboardingURIWithConfig(string(uri), string(config))
}

func GetStatusBytes() []byte {
	return []byte(mobilebridge.GetStatus())
}

func RuntimeMemoryStatsJSON() string {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	out := struct {
		Alloc        uint64 `json:"alloc"`
		HeapAlloc    uint64 `json:"heap_alloc"`
		HeapIdle     uint64 `json:"heap_idle"`
		HeapReleased uint64 `json:"heap_released"`
		NumGC        uint32 `json:"num_gc"`
		NumGoroutine int    `json:"num_goroutine"`
		GOMAXPROCS   int    `json:"gomaxprocs"`
	}{
		Alloc:        ms.Alloc,
		HeapAlloc:    ms.HeapAlloc,
		HeapIdle:     ms.HeapIdle,
		HeapReleased: ms.HeapReleased,
		NumGC:        ms.NumGC,
		NumGoroutine: runtime.NumGoroutine(),
		GOMAXPROCS:   runtime.GOMAXPROCS(0),
	}
	raw, _ := json.Marshal(out)
	return string(raw)
}

func StartLocalPprof(addr string) error {
	if addr == "" {
		addr = "127.0.0.1:0"
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	if host != "127.0.0.1" && host != "localhost" && host != "::1" {
		return fmt.Errorf("pprof listener must be loopback-only")
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	go func() {
		_ = http.Serve(ln, nil)
	}()
	return nil
}
