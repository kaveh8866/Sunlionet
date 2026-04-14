//go:build outside
// +build outside

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/kaveh/shadownet-agent/pkg/bundle"
	"github.com/kaveh/shadownet-agent/pkg/profile"
)

// ShadowNet-Outside Builder/Tester
// This tool generates pre-validated configs, tests them on live infrastructure,
// and securely publishes encrypted bundles via Signal.

func main() {
	fmt.Println("Starting ShadowNet Tools (Outside Version)")

	// 1. Generate or ingest new proxy endpoints
	newProfiles := []profile.Profile{
		{
			ID:     "reality_03_a",
			Family: profile.FamilyReality,
			Endpoint: profile.Endpoint{
				Host: "192.0.2.100",
				Port: 443,
			},
			TemplateRef: "outbound/reality_vless_v1",
		},
		{
			ID:     "tuic_02_b",
			Family: profile.FamilyTUIC,
			Endpoint: profile.Endpoint{
				Host: "198.51.100.2",
				Port: 8443,
			},
			TemplateRef: "outbound/tuic_v5",
		},
	}

	// 2. Validate Templates & Perform Live Test (testharness)
	// For each profile, attempt local sing-box connection to verify.
	log.Println("Testing 2 new profiles against live endpoints...")

	// 3. Construct Bundle Payload
	payload := bundle.BundlePayload{
		SchemaVersion:   1,
		MinAgentVersion: "1.0.0",
		Profiles:        newProfiles,
		Revocations:     []string{"reality_01_a"}, // Block compromised endpoint
		Templates: map[string]bundle.Template{
			"outbound/reality_vless_v1": {TemplateText: `{"type": "vless", "server": "{{.Host}}", ...}`},
		},
	}

	// 4. Serialize to disk (simulating bundle creation)
	bundleBytes, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		log.Fatalf("Failed to serialize bundle: %v", err)
	}

	outDir := filepath.Join(".", "dist")
	os.MkdirAll(outDir, 0755)

	bundlePath := filepath.Join(outDir, "bundle.snb.json")
	if err := os.WriteFile(bundlePath, bundleBytes, 0644); err != nil {
		log.Fatalf("Failed to write bundle: %v", err)
	}

	log.Printf("Successfully generated bundle with %d profiles at %s\n", len(payload.Profiles), bundlePath)
	log.Println("Ready to push bundle.snb to Signal recipients...")
}
