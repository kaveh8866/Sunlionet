//go:build inside
// +build inside

package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"filippo.io/age"
	"github.com/kaveh/shadownet-agent/pkg/detector"
	"github.com/kaveh/shadownet-agent/pkg/importctl"
	"github.com/kaveh/shadownet-agent/pkg/llm"
	"github.com/kaveh/shadownet-agent/pkg/policy"
	"github.com/kaveh/shadownet-agent/pkg/profile"
	"github.com/kaveh/shadownet-agent/pkg/sbctl"
	"github.com/kaveh/shadownet-agent/pkg/signalrx"
)

// ShadowNet-Inside Supervisor
// This process runs continuously, managing sing-box and monitoring network health.

func main() {
	fmt.Println("Starting ShadowNet Agent (Inside Version)")

	// 1. Load Encrypted Local Store
	// profiles.enc, state.db, etc.
	var importer *importctl.Importer
	{
		masterKey := os.Getenv("SHADOWNET_MASTER_KEY")
		if len(masterKey) == 32 {
			storePath := os.Getenv("SHADOWNET_STORE_PATH")
			if storePath == "" {
				home, err := os.UserHomeDir()
				if err == nil {
					storePath = filepath.Join(home, ".shadownet", "profiles.enc")
				} else {
					storePath = filepath.Join(".", "profiles.enc")
				}
			}

			store, err := profile.NewStore(storePath, masterKey)
			if err != nil {
				log.Printf("Signal import disabled: failed to init local store: %v", err)
			} else {
				signerPubB64URL := os.Getenv("SHADOWNET_TRUSTED_SIGNER_PUB_B64URL")
				ageIdentityStr := os.Getenv("SHADOWNET_AGE_IDENTITY")

				if signerPubB64URL == "" || ageIdentityStr == "" {
					log.Printf("Signal import disabled: missing SHADOWNET_TRUSTED_SIGNER_PUB_B64URL or SHADOWNET_AGE_IDENTITY")
				} else {
					pubBytes, err := base64.RawURLEncoding.DecodeString(signerPubB64URL)
					if err != nil {
						log.Printf("Signal import disabled: invalid SHADOWNET_TRUSTED_SIGNER_PUB_B64URL: %v", err)
					} else if len(pubBytes) != ed25519.PublicKeySize {
						log.Printf("Signal import disabled: invalid signer public key size: %d", len(pubBytes))
					} else {
						ageIdentity, err := age.ParseX25519Identity(ageIdentityStr)
						if err != nil {
							log.Printf("Signal import disabled: invalid SHADOWNET_AGE_IDENTITY: %v", err)
						} else {
							importer = importctl.NewImporter(store, []ed25519.PublicKey{ed25519.PublicKey(pubBytes)}, ageIdentity)
							log.Printf("Signal import enabled: store=%s", storePath)
						}
					}
				}
			}
		} else if masterKey != "" {
			log.Printf("Signal import disabled: SHADOWNET_MASTER_KEY must be exactly 32 bytes")
		} else {
			log.Printf("Signal import disabled: missing SHADOWNET_MASTER_KEY")
		}
	}

	activeProfile := profile.Profile{
		ID:     "reality_01_a",
		Family: profile.FamilyReality,
		Capabilities: profile.Capabilities{
			Transport: "tcp",
		},
	}

	// 2. Initialize Policy Engine & LLM Advisor
	engine := policy.Engine{
		MaxBurstFailures: 3,
	}

	advisor := &llm.LocalGGUFAdvisor{
		ModelPath: "/var/lib/shadownet/models/phi-4-mini-q4_k_m.gguf",
	}

	// 3. Start Sing-Box Controller
	sbController := sbctl.NewController("/var/lib/shadownet/config", "/usr/bin/sing-box")

	// Example template
	templateText := `{
		"outbounds": [{
			"type": "{{.Capabilities.Transport}}",
			"server": "{{.Endpoint.Host}}",
			"server_port": {{.Endpoint.Port}},
			"tag": "proxy"
		}]
	}`

	configJSON, err := sbController.GenerateConfig(activeProfile, templateText)
	if err != nil {
		log.Fatalf("Failed to generate initial config: %v", err)
	}

	if err := sbController.ApplyAndReload(configJSON); err != nil {
		log.Printf("Warning: Failed to start sing-box: %v", err)
	}
	defer sbController.Stop()

	// 4. Start Detector Subsystem
	eventChan := make(chan detector.Event, 100)
	worker := detector.NewWorker(eventChan)
	worker.Start(activeProfile)

	// 4.5 Start Signal Receiver
	rx := signalrx.NewReceiver(5 * time.Minute)
	rx.Start()

	// 5. Main Control Loop
	recentEvents := []detector.Event{}
	for {
		select {
		case uri := <-rx.URIChan:
			if importer == nil {
				log.Printf("Supervisor: Received bundle URI but Signal import is disabled")
				continue
			}
			payload, err := importer.ParseURI(uri)
			if err != nil {
				log.Printf("Supervisor: Rejected bundle: %v", err)
				continue
			}
			if err := importer.ProcessAndStore(payload); err != nil {
				log.Printf("Supervisor: Failed to store bundle payload: %v", err)
				continue
			}
			log.Printf("Supervisor: Imported bundle. Revocations: %d, New Profiles: %d", len(payload.Revocations), len(payload.Profiles))

		case ev := <-eventChan:
			log.Printf("Detected Event: [%s] %s\n", ev.Severity, ev.Type)
			recentEvents = append(recentEvents, ev)

			// Prune old events
			if len(recentEvents) > 10 {
				recentEvents = recentEvents[1:]
			}

			// Deterministic Policy Pass
			action := engine.Evaluate(recentEvents, activeProfile)

			// LLM Advisor Pass if needed
			if action.Type == policy.ActionInvokeLLM {
				log.Println("Signals ambiguous. Invoking LLM Advisor...")

				candidates := []profile.Profile{
					{ID: "tuic_02_b", Family: profile.FamilyTUIC},
					{ID: "shadowtls_01", Family: profile.FamilyShadowTLS},
				}

				llmAct, err := advisor.ProposeAction("fingerprint_hash_abc", activeProfile, candidates, recentEvents)
				if err != nil {
					log.Printf("LLM Advisor Failed: %v. Falling back to deterministic ladder.\n", err)
					action = policy.Action{Type: policy.ActionRollbackLastGood}
				} else {
					action = llmAct
				}
			}

			// Apply Action
			log.Printf("Applying Action: %s -> %s\n", action.Type, action.TargetProfile)
			if action.Type == policy.ActionSwitchProfile && action.TargetProfile != activeProfile.ID {
				// In a real scenario, we would lookup the target profile from the local DB
				activeProfile = profile.Profile{
					ID:           action.TargetProfile,
					Family:       profile.FamilyTUIC,
					Endpoint:     profile.Endpoint{Host: "new.target.com", Port: 443},
					Capabilities: profile.Capabilities{Transport: "quic"},
				}

				newConfigJSON, _ := sbController.GenerateConfig(activeProfile, templateText)
				sbController.ApplyAndReload(newConfigJSON)

				// Reset events after switch
				recentEvents = nil
			}

		case <-time.After(30 * time.Second):
			// Routine health check, cleanup
		}
	}
}
