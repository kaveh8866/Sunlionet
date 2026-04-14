package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kaveh/shadownet-agent/pkg/detector"
	"github.com/kaveh/shadownet-agent/pkg/llm"
	"github.com/kaveh/shadownet-agent/pkg/policy"
	"github.com/kaveh/shadownet-agent/pkg/profile"
	"github.com/kaveh/shadownet-agent/pkg/sbctl"
)

var (
	llamaURL     string
	storePath    string
	templateDir  string
	debugMode    bool
	batteryLevel int
)

func init() {
	flag.StringVar(&llamaURL, "llama-url", "http://127.0.0.1:8080", "Local llama.cpp API endpoint")
	flag.StringVar(&storePath, "store", "/var/lib/shadownet/store.enc", "Path to AES-GCM encrypted config DB")
	flag.StringVar(&templateDir, "templates", "/opt/shadownet/templates", "Path to sing-box JSON templates")
	flag.BoolVar(&debugMode, "debug", false, "Enable verbose logging and LLM reasoning output")
	flag.IntVar(&batteryLevel, "battery", 100, "Simulated battery level (Android)")
}

func main() {
	flag.Parse()

	log.Println("=== Starting ShadowNet Autonomous Agent ===")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 1. Initialize Encrypted Store
	store, err := profile.NewStore(storePath, "0123456789abcdef0123456789abcdef") // 32-byte key
	if err != nil {
		log.Fatalf("Failed to open encrypted store: %v", err)
	}

	// 2. Initialize sing-box Controller and Generator
	generator := sbctl.NewConfigGenerator(templateDir)
	controller := sbctl.NewController("/var/run/sing-box.sock", "")

	// 3. Initialize Local LLM Client (llama.cpp)
	advisor := llm.NewLocalLlamaCPPClient(llamaURL, debugMode)

	// 4. Initialize Rotation/Policy Manager
	rotationMgr := policy.NewRotationManager(advisor, controller, generator, store)

	// 5. Initialize Anomaly Channel
	anomalyChan := make(chan detector.Event, 100)

	// Start OS Signal listener for graceful shutdown
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		log.Println("Received termination signal. Shutting down gracefully...")
		cancel()
	}()

	// Start Rotation Manager in background
	go rotationMgr.Start(ctx, anomalyChan)

	// ==========================================
	// MAIN AGENT LOOP (Detection Suite)
	// ==========================================

	// Determine detection frequency based on Android battery rules
	detectInterval := 8 * time.Second
	if batteryLevel < 20 {
		log.Println("[POWER] Battery < 20%. Throttling detection suite to 30s intervals.")
		detectInterval = 30 * time.Second
	}

	ticker := time.NewTicker(detectInterval)
	defer ticker.Stop()

	log.Printf("Agent Loop running every %v", detectInterval)

	for {
		select {
		case <-ctx.Done():
			log.Println("Agent Loop terminated.")
			return
		case <-ticker.C:
			runDetectionSuite(anomalyChan)
		}
	}
}

// runDetectionSuite runs the fast active+passive checks (must take <5s total)
func runDetectionSuite(anomalies chan<- detector.Event) {
	// Passive check (virtually instant, 0 network overhead)
	retransRatio, err := detector.PassiveTCPStats()
	if err == nil && retransRatio > 0.20 {
		anomalies <- detector.Event{
			Type:      "HIGH_RETRANSMISSION",
			Severity:  "medium",
			Timestamp: time.Now(),
		}
	}

	// Active check 1: Baseline Ping
	ok, err := detector.CheckConnectivityBaseline()
	if !ok {
		anomalies <- detector.Event{
			Type:      "TOTAL_BLACKOUT_SUSPECTED",
			Severity:  "critical",
			Timestamp: time.Now(),
		}
		return // Skip further checks if gateway is unreachable
	}

	// Active check 2: SNI Reset (only if current outbound is TCP/Reality)
	// In a real system, we check rm.currentConfig.Protocol
	sniBlocked, _ := detector.CheckSNIReset("www.apple.com", "1.1.1.1")
	if sniBlocked {
		anomalies <- detector.Event{
			Type:      "SNI_BLOCK_SUSPECTED",
			Severity:  "high",
			Timestamp: time.Now(),
		}
	}

	// Active check 3: UDP Dropping (only if current outbound is UDP/Hysteria)
	udpBlocked, _ := detector.CheckUDPBlocked("1.1.1.1:53")
	if udpBlocked {
		anomalies <- detector.Event{
			Type:      "UDP_BLOCK_SUSPECTED",
			Severity:  "high",
			Timestamp: time.Now(),
		}
	}
}
