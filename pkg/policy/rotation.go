package policy

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/kaveh/shadownet-agent/pkg/detector"
	"github.com/kaveh/shadownet-agent/pkg/profile"
	"github.com/kaveh/shadownet-agent/pkg/sbctl"
)

// RotationManager handles the 8-20m config rotation and DPI anomaly handling
type RotationManager struct {
	llmAdvisor Advisor
	controller *sbctl.Controller
	generator  *sbctl.ConfigGenerator
	store      *profile.Store

	currentConfig  sbctl.LLMDecision
	currentProfile profile.Profile

	// Temporary Blacklist: ProfileID -> Penalty Expiry Time
	blacklist map[string]time.Time
	mu        sync.RWMutex
}

func NewRotationManager(
	llm Advisor,
	ctrl *sbctl.Controller,
	gen *sbctl.ConfigGenerator,
	store *profile.Store,
) *RotationManager {
	return &RotationManager{
		llmAdvisor: llm,
		controller: ctrl,
		generator:  gen,
		store:      store,
		blacklist:  make(map[string]time.Time),
	}
}

// Start initiates the rotation loop
func (rm *RotationManager) Start(ctx context.Context, anomalies <-chan detector.Event) {
	// Initial rotation on startup
	rm.Rotate(ctx, nil)

	// Default rotation timer (will be updated by LLM)
	rotationInterval := 10 * time.Minute
	ticker := time.NewTicker(rotationInterval)
	defer ticker.Stop()

	recentAnomalies := make([]detector.Event, 0)
	anomalyThreshold := 3
	anomalyWindow := 60 * time.Second

	for {
		select {
		case <-ctx.Done():
			log.Println("RotationManager shutting down")
			return

		case event := <-anomalies:
			log.Printf("RotationManager received anomaly: %s", event.Type)
			recentAnomalies = append(recentAnomalies, event)

			// Clean up old anomalies
			cutoff := time.Now().Add(-anomalyWindow)
			var validAnomalies []detector.Event
			for _, e := range recentAnomalies {
				if e.Timestamp.After(cutoff) {
					validAnomalies = append(validAnomalies, e)
				}
			}
			recentAnomalies = validAnomalies

			// Trigger emergency rotation if DPI threshold met
			if len(recentAnomalies) >= anomalyThreshold {
				log.Println("DPI threshold exceeded! Forcing emergency rotation...")

				// Blacklist current profile for 60 minutes
				rm.mu.Lock()
				rm.blacklist[rm.currentProfile.ID] = time.Now().Add(60 * time.Minute)
				rm.mu.Unlock()

				rm.Rotate(ctx, recentAnomalies)
				recentAnomalies = nil // reset
				ticker.Reset(time.Duration(rm.currentConfig.RotationIntervalSec) * time.Second)
			}

		case <-ticker.C:
			log.Println("Scheduled rotation tick triggered")
			rm.Rotate(ctx, nil)
			ticker.Reset(time.Duration(rm.currentConfig.RotationIntervalSec) * time.Second)
		}
	}
}

// Rotate asks the LLM for a new decision and hot-reloads sing-box
func (rm *RotationManager) Rotate(ctx context.Context, recentEvents []detector.Event) {
	candidates, err := rm.getHealthyCandidates()
	if err != nil {
		log.Printf("Failed to load seed configs: %v", err)
		return
	}

	if len(candidates) == 0 {
		log.Println("CRITICAL: No healthy seed configs remaining. Falling back to emergency Bluetooth Mesh / DNS Tunnel.")
		// Implementation would trigger Bluetooth scanning mode here
		return
	}

	// 1. Invoke LLM for next strategy
	// Passing a simulated fingerprint and candidates
	_, err = rm.llmAdvisor.ProposeAction("AS43089-MCI", rm.currentProfile, candidates, recentEvents)
	if err != nil {
		log.Printf("LLM failed to propose action: %v. Using deterministic fallback.", err)
		// Deterministic Fallback: Just pick the first healthy candidate
		rm.currentProfile = candidates[0]
		// Mock a decision
		rm.currentConfig = sbctl.LLMDecision{
			Protocol:            string(rm.currentProfile.Family),
			RotationIntervalSec: 600,
		}
	} else {
		// (Assume Action is converted to LLMDecision)
		// Here we would unmarshal the LLM string output into sbctl.LLMDecision
		// For this skeleton, we assume the LLM output is parsed.
		rm.currentProfile = candidates[0] // Simplify selection mapping
	}

	// 2. Generate new config
	configBytes, err := rm.generator.Generate(rm.currentConfig, rm.currentProfile.Endpoint.Host, "uuid", "pubkey")
	if err != nil {
		log.Printf("Config generation failed")
		return
	}

	// 3. Apply via hot-reload
	if err := rm.controller.ApplyAndReload(string(configBytes)); err != nil {
		log.Printf("Hot-reload failed! Blacklisting profile %s.", rm.currentProfile.ID)

		rm.mu.Lock()
		rm.blacklist[rm.currentProfile.ID] = time.Now().Add(30 * time.Minute)
		rm.mu.Unlock()
		return
	}

	log.Printf("Successfully rotated to protocol %s. Next rotation in %d seconds.",
		rm.currentConfig.Protocol, rm.currentConfig.RotationIntervalSec)
}

// getHealthyCandidates filters stored seeds against the temporary blacklist
func (rm *RotationManager) getHealthyCandidates() ([]profile.Profile, error) {
	allProfiles, err := rm.store.Load()
	if err != nil {
		return nil, err
	}

	var healthy []profile.Profile
	now := time.Now()

	rm.mu.Lock()
	for _, p := range allProfiles {
		expiry, blacklisted := rm.blacklist[p.ID]
		if blacklisted {
			if now.Before(expiry) {
				continue
			}
			delete(rm.blacklist, p.ID)
		}
		healthy = append(healthy, p)
	}
	rm.mu.Unlock()

	return healthy, nil
}
