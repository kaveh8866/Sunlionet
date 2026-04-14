package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"text/template"

	"github.com/kaveh/shadownet-agent/pkg/detector"
	"github.com/kaveh/shadownet-agent/pkg/policy"
	"github.com/kaveh/shadownet-agent/pkg/profile"
)

const sysPrompt = `You are a "Censorship Evasion Strategist", an expert AI agent operating in a highly restrictive DPI environment (Iran). 
Your objective is to keep the user's internet connection alive with ZERO user intervention.
You are paranoid about DPI fingerprinting and prefer diversity in outbound configs to prevent active probing and blocklisting.

The fallback chain must be strictly ordered as follows:
1. Fast obfuscated TLS (Reality+uTLS, ShadowTLS)
2. QUIC-based / UDP (Hysteria2, TUIC)
3. DNS tunnel fallback (Slow, reliable)
4. Local mesh (Total blackout)

Output ONLY valid JSON matching this schema, with no markdown or backticks:
{
  "protocol": "<reality | hysteria2 | tuic | shadowtls | dns_tunnel | hybrid>",
  "parameters": {
    "sni": "<facade domain, e.g. www.apple.com>",
    "utls": "<chrome | firefox | safari>",
    "port": <int>
  },
  "rotation_interval_sec": <int>,
  "enable_bt_mesh": <bool>,
  "explanation": "<short human-readable debugging summary>"
}

### FEW-SHOT EXAMPLES ###
Input: {"recent_failures": ["SNI_BLOCK_SUSPECTED"], "active_outbound": "reality"}
Output: {"protocol": "reality", "parameters": {"sni": "www.microsoft.com", "utls": "firefox", "port": 443}, "rotation_interval_sec": 1800, "enable_bt_mesh": false, "explanation": "Reality SNI blocked. Rotating to microsoft facade and changing uTLS fingerprint to evade DPI signature."}

Input: {"recent_failures": ["UDP_BLOCK_SUSPECTED", "THROUGHPUT_COLLAPSE"], "active_outbound": "hysteria2"}
Output: {"protocol": "shadowtls", "parameters": {"sni": "gateway.icloud.com", "utls": "chrome", "port": 443}, "rotation_interval_sec": 3600, "enable_bt_mesh": false, "explanation": "Severe UDP blocking detected. Moving up fallback chain to ShadowTLS (TCP) to bypass UDP shaping."}

Input: {"recent_failures": ["DNS_POISON_SUSPECTED", "ACTIVE_RESET_SUSPECTED"], "active_outbound": "reality"}
Output: {"protocol": "dns_tunnel", "parameters": {"sni": "", "utls": "", "port": 53}, "rotation_interval_sec": 7200, "enable_bt_mesh": true, "explanation": "Severe DPI interference and active resets. Falling back to emergency DNS tunnel and activating Bluetooth mesh discovery."}

### CURRENT STATE ###
Current Network Fingerprint: {{.Fingerprint}}
Current Profile: {{.CurrentProfile.ID}} ({{.CurrentProfile.Family}})
System Resources: CPU {{.CPU}}%, RAM {{.RAM}}%, Battery {{.Battery}}%

Candidates:
{{range .Candidates}}
- ID: {{.ID}} | Family: {{.Family}} | Transport: {{.Capabilities.Transport}} | Score: {{.Health.Score}}
{{end}}

Recent Events (DPI Detections):
{{range .RecentEvents}}
- {{.Timestamp.Format "15:04:05"}}: [{{.Severity}}] {{.Type}}
{{end}}
`

// LocalGGUFAdvisor wraps the local llama.cpp execution (e.g. Phi-4-mini Q4_K_M)
type LocalGGUFAdvisor struct {
	ModelPath string
}

func (a *LocalGGUFAdvisor) ProposeAction(
	fingerprint string,
	currentProfile profile.Profile,
	candidates []profile.Profile,
	recentEvents []detector.Event,
) (policy.Action, error) {

	// 1. Construct the prompt
	tmpl, err := template.New("prompt").Parse(sysPrompt)
	if err != nil {
		return policy.Action{}, err
	}

	data := struct {
		Fingerprint    string
		CurrentProfile profile.Profile
		Candidates     []profile.Profile
		RecentEvents   []detector.Event
		CPU            int
		RAM            int
		Battery        int
	}{
		Fingerprint:    fingerprint,
		CurrentProfile: currentProfile,
		Candidates:     candidates,
		RecentEvents:   recentEvents,
		CPU:            45, // Mocked for demonstration
		RAM:            60, // Mocked for demonstration
		Battery:        80, // Mocked for demonstration
	}

	var promptBuf bytes.Buffer
	if err := tmpl.Execute(&promptBuf, data); err != nil {
		return policy.Action{}, err
	}

	log.Printf("llm: Generated Prompt:\n%s", promptBuf.String())

	// 2. Invoke local CGO/llama.cpp binding (mocked here)
	// In a real system, you would pass `promptBuf.String()` to llama.cpp with a JSON grammar constraint.

	// Example mock output that the LLM should generate:
	mockJSONOutput := `{
		"action": "SWITCH_PROFILE",
		"target_profile_id": "tuic_02_b",
		"mutation_set": "tuic_sni_port_rotate_1",
		"cooldown_sec": 900,
		"reason_code": "udp_ok_hysteria_degraded",
		"confidence": 0.82
	}`

	var act policy.Action
	if err := json.Unmarshal([]byte(mockJSONOutput), &act); err != nil {
		return policy.Action{}, fmt.Errorf("llm produced invalid json: %w", err)
	}

	// Validate the LLM choice is within candidates
	valid := false
	for _, c := range candidates {
		if c.ID == act.TargetProfile {
			valid = true
			break
		}
	}
	if !valid {
		return policy.Action{}, fmt.Errorf("llm hallucinated profile: %s", act.TargetProfile)
	}

	return act, nil
}
