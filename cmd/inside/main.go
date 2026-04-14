//go:build inside
// +build inside

package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/kaveh/shadownet-agent/pkg/detector"
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
		case bundlePayload := <-rx.BundleChan:
			log.Printf("Supervisor: Processing new bundle! Revocations: %d, New Profiles: %d",
				len(bundlePayload.Revocations), len(bundlePayload.Profiles))
			// Apply new profiles logic...

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
