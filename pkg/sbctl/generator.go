package sbctl

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"

	"github.com/kaveh/shadownet-agent/pkg/profile"
)

// LLMDecision represents the structured output from the LLM
type LLMDecision struct {
	Protocol   string `json:"protocol"`
	Parameters struct {
		SNI  string `json:"sni"`
		UTLS string `json:"utls"`
		Port int    `json:"port"`
	} `json:"parameters"`
	RotationIntervalSec int    `json:"rotation_interval_sec"`
	EnableBTMesh        bool   `json:"enable_bt_mesh"`
	Explanation         string `json:"explanation"`
}

// ConfigGenerator dynamically generates full sing-box configs
type ConfigGenerator struct {
	templateDir string
	baseConfig  string
}

func NewConfigGenerator(templateDir string) *ConfigGenerator {
	// Base sing-box config containing DNS, Log, Inbounds, Routing
	baseConfig := `{
		"log": { "level": "error", "timestamp": true },
		"dns": {
			"servers": [
				{ "tag": "remote", "address": "https://1.1.1.1/dns-query", "detour": "proxy" },
				{ "tag": "local", "address": "local", "detour": "direct" }
			],
			"rules": [ { "outbound": "any", "server": "local" } ]
		},
		"inbounds": [
			{
				"type": "tun",
				"tag": "tun-in",
				"interface_name": "tun0",
				"inet4_address": "172.19.0.1/30",
				"auto_route": true,
				"strict_route": true,
				"stack": "system",
				"sniff": true
			}
		],
		"outbounds": [
			%s,
			{ "type": "direct", "tag": "direct" },
			{ "type": "block", "tag": "block" }
		],
		"route": {
			"rules": [
				{ "ip_is_private": true, "outbound": "direct" }
			],
			"final": "proxy"
		}
	}`

	return &ConfigGenerator{
		templateDir: templateDir,
		baseConfig:  baseConfig,
	}
}

// Generate assembles the LLM parameters and the base template into a valid sing-box config
func (g *ConfigGenerator) Generate(decision LLMDecision, serverIP string, uuid string, pubKey string) ([]byte, error) {
	tmplPath := filepath.Join(g.templateDir, decision.Protocol+".json")
	tmplContent, err := os.ReadFile(tmplPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read protocol template %s: %w", decision.Protocol, err)
	}

	tmpl, err := template.New(decision.Protocol).Parse(string(tmplContent))
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	// Generate randomizations for fingerprint defense
	shortID := generateRandomHex(8)

	data := profile.Profile{
		Family:  profile.Family(decision.Protocol),
		Enabled: true,
		Endpoint: profile.Endpoint{
			Host: serverIP,
			Port: decision.Parameters.Port,
		},
		Credentials: profile.Credentials{
			UUID:            uuid,
			Password:        uuid,
			PublicKey:       pubKey,
			ShortID:         shortID,
			SNI:             decision.Parameters.SNI,
			UTLSFingerprint: decision.Parameters.UTLS,
			ObfsPassword:    shortID,
		},
		MutationPolicy: profile.MutationPolicy{
			AllowedSets:         nil,
			MaxMutationsPerHour: 0,
		},
	}

	var outboundBuf bytes.Buffer
	if err := tmpl.Execute(&outboundBuf, data); err != nil {
		return nil, fmt.Errorf("failed to execute template: %w", err)
	}

	// Inject the specific outbound into the base config array
	fullConfigStr := fmt.Sprintf(g.baseConfig, outboundBuf.String())

	// Format JSON nicely
	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, []byte(fullConfigStr), "", "  "); err != nil {
		return nil, fmt.Errorf("generated invalid JSON: %w\n%s", err, fullConfigStr)
	}

	return prettyJSON.Bytes(), nil
}

// Validate uses 'sing-box check' to ensure the config is syntactically valid before applying
func (g *ConfigGenerator) Validate(configBytes []byte) error {
	tmpFile, err := os.CreateTemp("", "singbox-check-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(configBytes); err != nil {
		return err
	}
	tmpFile.Close()

	cmd := exec.Command("sing-box", "check", "-c", tmpFile.Name())
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("sing-box check failed: %v\nOutput: %s", err, string(output))
	}
	return nil
}

func generateRandomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}
