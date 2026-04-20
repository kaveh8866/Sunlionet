//go:build inside && daemon
// +build inside,daemon

package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/kaveh/shadownet-agent/pkg/chat"
	"github.com/kaveh/shadownet-agent/pkg/detector"
	"github.com/kaveh/shadownet-agent/pkg/identity"
	"github.com/kaveh/shadownet-agent/pkg/llm"
	"github.com/kaveh/shadownet-agent/pkg/messaging"
	"github.com/kaveh/shadownet-agent/pkg/policy"
	"github.com/kaveh/shadownet-agent/pkg/profile"
	"github.com/kaveh/shadownet-agent/pkg/relay"
	"github.com/kaveh/shadownet-agent/pkg/sbctl"
)

var (
	llamaURL     string
	storePath    string
	templateDir  string
	masterKeyArg string
	debugMode    bool
	batteryLevel int

	relayURL           string
	identityStorePath  string
	mailboxRotationSec int
	decoyMailboxCount  int
)

func init() {
	flag.StringVar(&llamaURL, "llama-url", "http://127.0.0.1:8080", "Local llama.cpp API endpoint")
	flag.StringVar(&storePath, "store", "/var/lib/sunlionet/store.enc", "Path to AES-GCM encrypted config DB (legacy default was /var/lib/shadownet/store.enc)")
	flag.StringVar(&templateDir, "templates", "/opt/sunlionet/templates", "Path to sing-box JSON templates (legacy default was /opt/shadownet/templates)")
	flag.StringVar(&masterKeyArg, "master-key", "", "Master key for local encrypted storage (32 raw bytes, 64 hex chars, or base64/base64url encoding of 32 bytes)")
	flag.BoolVar(&debugMode, "debug", false, "Enable verbose logging and LLM reasoning output")
	flag.IntVar(&batteryLevel, "battery", 100, "Simulated battery level (Android)")
	flag.StringVar(&relayURL, "relay-url", "", "Relay base URL (e.g. https://relay.example.com)")
	flag.StringVar(&identityStorePath, "identity-store", "", "Path to AES-GCM encrypted identity DB (defaults to <store dir>/identity.enc)")
	flag.IntVar(&mailboxRotationSec, "mailbox-rotation-sec", 3600, "Mailbox rotation period in seconds")
	flag.IntVar(&decoyMailboxCount, "decoy-mailboxes", 2, "Number of decoy mailboxes to poll per persona")
}

func main() {
	flag.Parse()

	log.Println("=== Starting SunLionet Autonomous Agent ===")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if storePath == "/var/lib/sunlionet/store.enc" {
		if _, err := os.Stat(storePath); err != nil {
			if _, legacyErr := os.Stat("/var/lib/shadownet/store.enc"); legacyErr == nil {
				storePath = "/var/lib/shadownet/store.enc"
			}
		}
	}
	if templateDir == "/opt/sunlionet/templates" {
		if _, err := os.Stat(templateDir); err != nil {
			if _, legacyErr := os.Stat("/opt/shadownet/templates"); legacyErr == nil {
				templateDir = "/opt/shadownet/templates"
			}
		}
	}

	if masterKeyArg == "" {
		masterKeyArg = os.Getenv("SUNLIONET_MASTER_KEY")
		if masterKeyArg == "" {
			masterKeyArg = os.Getenv("SHADOWNET_MASTER_KEY")
		}
	}
	if relayURL == "" {
		relayURL = os.Getenv("SUNLIONET_RELAY_URL")
		if relayURL == "" {
			relayURL = os.Getenv("SHADOWNET_RELAY_URL")
		}
	}
	masterKey, err := profile.ParseMasterKey(masterKeyArg)
	if err != nil {
		log.Fatalf("Missing or invalid master key: %v (set --master-key, SUNLIONET_MASTER_KEY, or SHADOWNET_MASTER_KEY)", err)
	}

	// 1. Initialize Encrypted Store
	store, err := profile.NewStore(storePath, masterKey)
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

	if identityStorePath == "" {
		identityStorePath = filepath.Join(filepath.Dir(storePath), "identity.enc")
	}
	if relayURL != "" {
		idStore, err := identity.NewStore(identityStorePath, masterKey)
		if err != nil {
			log.Printf("Identity store init failed: %v", err)
		} else {
			idState, err := idStore.Load()
			if err != nil {
				log.Printf("Identity store load failed: %v", err)
			} else if len(idState.Personas) == 0 {
				log.Printf("No personas configured, skipping relay pollers")
			} else {
				chatStorePath := filepath.Join(filepath.Dir(storePath), "chat.enc")
				chatStore, err := chat.NewStore(chatStorePath, masterKey)
				if err != nil {
					log.Printf("Chat store init failed: %v", err)
				}
				var chatSvc *chat.Service
				if chatStore != nil {
					chatSvc, err = chat.NewService(chatStore)
					if err != nil {
						log.Printf("Chat service init failed: %v", err)
					}
				}
				rclient := relay.NewHTTPClient(relayURL)
				var idMu sync.Mutex
				for i := range idState.Personas {
					personaID := idState.Personas[i].ID
					persona := idState.Personas[i]
					b, created, err := idState.EnsureMailboxBinding(personaID)
					if err != nil {
						log.Printf("Mailbox binding init failed for persona %s: %v", personaID, err)
						continue
					}
					if created {
						_ = idStore.Save(idState)
					}
					binding := *b
					go runPersonaPollerRotationLoop(ctx, rclient, idStore, idState, &idMu, chatSvc, &persona, personaID, binding, mailboxRotationSec, decoyMailboxCount)
				}
			}
		}
	}

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

func runPersonaPollerRotationLoop(
	ctx context.Context,
	r relay.Relay,
	idStore *identity.Store,
	idState *identity.State,
	idMu *sync.Mutex,
	chatSvc *chat.Service,
	persona *identity.Persona,
	personaID identity.PersonaID,
	binding identity.MailboxBinding,
	rotationSec int,
	decoyCount int,
) {
	if r == nil || idStore == nil || idState == nil || idMu == nil {
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		now := time.Now()
		mailbox, nextRotateAt, err := binding.MailboxAt(now, int64(rotationSec))
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}
		prevMailbox, _ := binding.MailboxPrevAt(now, int64(rotationSec))
		decoys, _ := binding.DecoyMailboxesAt(now, int64(rotationSec), decoyCount)
		decoyMBs := make([]relay.MailboxID, 0, 1+len(decoys))
		if prevMailbox != "" && prevMailbox != mailbox {
			decoyMBs = append(decoyMBs, relay.MailboxID(prevMailbox))
		}
		for i := range decoys {
			if decoys[i] == "" || decoys[i] == mailbox {
				continue
			}
			decoyMBs = append(decoyMBs, relay.MailboxID(decoys[i]))
		}

		waitSec, _ := binding.DeriveInt("poll:wait_sec", 10, 20)
		cycleSec := waitSec
		jitterMs, _ := binding.DeriveInt("poll:jitter_ms", 300, 2200)
		backoffBase, _ := binding.DeriveInt("poll:backoff_base_ms", 200, 600)
		backoffMax, _ := binding.DeriveInt("poll:backoff_max_ms", 4000, 12000)
		ackDelay, _ := binding.DeriveInt("poll:ack_delay_ms", 150, 900)
		ackBatchSec, _ := binding.DeriveInt("poll:ack_batch_sec", 3, 12)
		ackBatchMax, _ := binding.DeriveInt("poll:ack_batch_max", 20, 80)
		coverAckSec, _ := binding.DeriveInt("poll:cover_ack_sec", 25, 90)
		coverAckCount, _ := binding.DeriveInt("poll:cover_ack_count", 1, 3)
		coverAckJitterMs, _ := binding.DeriveInt("poll:cover_ack_jitter_ms", 250, 1500)
		decoyPullSec, _ := binding.DeriveInt("poll:decoy_pull_sec", 10, 40)
		decoyWaitSec, _ := binding.DeriveInt("poll:decoy_wait_sec", 1, 5)
		decoyLimit, _ := binding.DeriveInt("poll:decoy_limit", 1, 2)
		decoyPullJitterMs, _ := binding.DeriveInt("poll:decoy_pull_jitter_ms", 250, 1500)

		pushMinDelayMs, _ := binding.DeriveInt("push:min_delay_ms", 200, 1200)
		pushMaxDelayMs, _ := binding.DeriveInt("push:max_delay_ms", 1500, 7000)
		pushBatchMax, _ := binding.DeriveInt("push:batch_max", 2, 10)
		pushInterJitterMs, _ := binding.DeriveInt("push:inter_jitter_ms", 50, 400)

		activeRelay := r
		shaped, err := relay.NewShapedRelay(r, relay.SendOptions{
			MinDelayMs:        pushMinDelayMs,
			MaxDelayMs:        pushMaxDelayMs,
			BatchMax:          pushBatchMax,
			QueueMax:          1000,
			InterPushJitterMs: pushInterJitterMs,
		})
		if err == nil && shaped != nil {
			activeRelay = shaped
		}

		childCtx, cancel := context.WithCancel(ctx)
		errCh := make(chan error, 1)
		p := &relay.Poller{
			Relay: activeRelay,
			Opts: relay.PollOptions{
				Mailbox:           relay.MailboxID(mailbox),
				Limit:             50,
				WaitSec:           waitSec,
				CycleSec:          cycleSec,
				JitterMs:          jitterMs,
				BackoffBaseMs:     backoffBase,
				BackoffMaxMs:      backoffMax,
				Ack:               true,
				AckDelayMsMax:     ackDelay,
				AckBatchSec:       ackBatchSec,
				AckBatchMax:       ackBatchMax,
				CoverAckSec:       coverAckSec,
				CoverAckCount:     coverAckCount,
				CoverAckJitterMs:  coverAckJitterMs,
				DecoyMailboxes:    decoyMBs,
				DecoyPullSec:      decoyPullSec,
				DecoyWaitSec:      decoyWaitSec,
				DecoyLimit:        decoyLimit,
				DecoyPullJitterMs: decoyPullJitterMs,
				Handle: func(ctx context.Context, msgs []relay.Message) error {
					handleRelayMessages(ctx, activeRelay, idStore, idState, idMu, chatSvc, persona, personaID, msgs)
					return nil
				},
			},
		}
		go func() { errCh <- p.Run(childCtx) }()

		sleepDur := time.Until(nextRotateAt)
		if sleepDur < 1*time.Second {
			sleepDur = 1 * time.Second
		}
		select {
		case <-ctx.Done():
			cancel()
			<-errCh
			if shaped != nil {
				_ = shaped.Close()
			}
			return
		case <-time.After(sleepDur):
			cancel()
			<-errCh
			if shaped != nil {
				_ = shaped.Close()
			}
		case <-errCh:
			cancel()
			if shaped != nil {
				_ = shaped.Close()
			}
		}
	}
}

func handleRelayMessages(ctx context.Context, r relay.Relay, store *identity.Store, state *identity.State, mu *sync.Mutex, chatSvc *chat.Service, persona *identity.Persona, personaID identity.PersonaID, msgs []relay.Message) {
	if store == nil || state == nil || len(msgs) == 0 {
		return
	}
	changed := false
	now := time.Now()
	for mi := range msgs {
		envStr := string(msgs[mi].Envelope)
		env, err := messaging.DecodeEnvelope(envStr)
		if err != nil {
			continue
		}
		var plaintext []byte
		mu.Lock()
		for i := range state.PreKeys {
			priv, err := state.PreKeys[i].DecodePrivate()
			if err != nil {
				continue
			}
			pt, _, err := messaging.DecryptWithPreKey(env, priv)
			if err != nil {
				continue
			}
			plaintext = pt
			state.PreKeys = append(state.PreKeys[:i], state.PreKeys[i+1:]...)
			state.UpdatedAt = now.Unix()
			changed = true
			break
		}
		mu.Unlock()
		if chatSvc != nil && persona != nil && len(plaintext) > 0 {
			_ = chatSvc.ApplyIncomingWithRelay(ctx, r, persona, msgs[mi], envStr, plaintext)
		}
	}
	if changed {
		mu.Lock()
		_ = store.Save(state)
		mu.Unlock()
	}
	_ = personaID
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
