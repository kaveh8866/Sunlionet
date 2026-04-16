package sbctl

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/kaveh/shadownet-agent/pkg/profile"
)

type RenderResult struct {
	ConfigPath   string
	ConfigBytes  []byte
	OutboundJSON json.RawMessage
}

func RenderConfigToFile(prof profile.Profile, templateText string, outPath string) (*RenderResult, error) {
	outbound, err := renderOutbound(prof, templateText)
	if err != nil {
		return nil, err
	}
	cfgBytes, err := buildBaseConfig(outbound)
	if err != nil {
		return nil, err
	}
	if err := writeFileAtomic(outPath, cfgBytes, 0o600); err != nil {
		return nil, err
	}
	return &RenderResult{
		ConfigPath:   outPath,
		ConfigBytes:  cfgBytes,
		OutboundJSON: outbound,
	}, nil
}

func renderOutbound(prof profile.Profile, templateText string) (json.RawMessage, error) {
	tmpl, err := template.New("outbound").Parse(templateText)
	if err != nil {
		return nil, fmt.Errorf("failed to parse outbound template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, prof); err != nil {
		return nil, fmt.Errorf("failed to execute outbound template: %w", err)
	}
	out := bytes.TrimSpace(buf.Bytes())
	if !json.Valid(out) {
		return nil, fmt.Errorf("rendered outbound is not valid json")
	}
	return json.RawMessage(out), nil
}

func buildBaseConfig(outbound json.RawMessage) ([]byte, error) {
	type config struct {
		Log       map[string]any    `json:"log"`
		DNS       map[string]any    `json:"dns"`
		Inbounds  []map[string]any  `json:"inbounds"`
		Outbounds []json.RawMessage `json:"outbounds"`
		Route     map[string]any    `json:"route"`
	}

	cfg := config{
		Log: map[string]any{
			"level":     "error",
			"timestamp": true,
		},
		DNS: map[string]any{
			"servers": []map[string]any{
				{"tag": "remote", "address": "https://1.1.1.1/dns-query", "detour": "proxy"},
				{"tag": "local", "address": "local", "detour": "direct"},
			},
			"rules": []map[string]any{
				{"outbound": "any", "server": "local"},
			},
		},
		Inbounds: []map[string]any{
			{
				"type":           "tun",
				"tag":            "tun-in",
				"interface_name": "tun0",
				"inet4_address":  "172.19.0.1/30",
				"auto_route":     true,
				"strict_route":   true,
				"stack":          "system",
				"sniff":          true,
			},
		},
		Outbounds: []json.RawMessage{
			outbound,
			json.RawMessage(`{"type":"direct","tag":"direct"}`),
			json.RawMessage(`{"type":"block","tag":"block"}`),
		},
		Route: map[string]any{
			"rules": []map[string]any{
				{"ip_is_private": true, "outbound": "direct"},
			},
			"final": "proxy",
		},
	}

	raw, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, raw, "", "  "); err != nil {
		return nil, err
	}
	return pretty.Bytes(), nil
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
