package sbctl

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"text/template"

	"github.com/kaveh/shadownet-agent/pkg/profile"
)

type RenderResult struct {
	ConfigPath   string
	ConfigBytes  []byte
	OutboundJSON json.RawMessage
}

func RenderConfigToFile(prof profile.Profile, templateText string, outPath string) (*RenderResult, error) {
	return RenderConfigToFileWithOptions(prof, templateText, outPath, RenderOptions{})
}

type RenderOptions struct {
	ProbeListenAddr    string
	DisableTun         bool
	Android            bool
	AllowPrivateBypass bool
}

func RenderConfigToFileWithOptions(prof profile.Profile, templateText string, outPath string, opts RenderOptions) (*RenderResult, error) {
	outbound, err := renderOutbound(prof, templateText)
	if err != nil {
		return nil, err
	}
	cfgBytes, err := buildBaseConfig(outbound, opts)
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
	if err := validateProfileForRender(prof); err != nil {
		return nil, err
	}
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
	validated, err := validateRenderedOutbound(out)
	if err != nil {
		return nil, err
	}
	return validated, nil
}

func buildBaseConfig(outbound json.RawMessage, opts RenderOptions) ([]byte, error) {
	type config struct {
		Log       map[string]any    `json:"log"`
		DNS       map[string]any    `json:"dns"`
		Inbounds  []map[string]any  `json:"inbounds"`
		Outbounds []json.RawMessage `json:"outbounds"`
		Route     map[string]any    `json:"route"`
	}

	isAndroid := opts.Android || runtime.GOOS == "android"

	tunInbound := map[string]any{
		"type":           "tun",
		"tag":            "tun-in",
		"interface_name": "tun0",
		"inet4_address":  "172.19.0.1/30",
		"auto_route":     true,
		"strict_route":   true,
		"stack":          "system",
		"sniff":          true,
	}
	routeCfg := map[string]any{
		"rules": []map[string]any{},
		"final": "proxy",
	}
	if opts.AllowPrivateBypass {
		routeCfg["rules"] = []map[string]any{
			{"ip_is_private": true, "outbound": "direct"},
		}
	}
	if isAndroid {
		tunInbound["auto_route"] = false
		tunInbound["strict_route"] = false
		routeCfg["override_android_vpn"] = true
	}

	inbounds := make([]map[string]any, 0, 2)
	if !opts.DisableTun {
		inbounds = append(inbounds, tunInbound)
	}
	if opts.ProbeListenAddr != "" {
		host, port, err := parseListenAddr(opts.ProbeListenAddr)
		if err != nil {
			return nil, err
		}
		inbounds = append(inbounds, map[string]any{
			"type":        "mixed",
			"tag":         "probe-in",
			"listen":      host,
			"listen_port": port,
		})
	}

	cfg := config{
		Log: map[string]any{
			"level":     "error",
			"timestamp": true,
		},
		DNS: map[string]any{
			"servers": []map[string]any{
				{"tag": "remote", "address": "https://1.1.1.1/dns-query", "detour": "proxy"},
			},
			"rules": []map[string]any{
				{"outbound": "any", "server": "remote"},
			},
		},
		Inbounds: inbounds,
		Outbounds: []json.RawMessage{
			outbound,
			json.RawMessage(`{"type":"direct","tag":"direct"}`),
			json.RawMessage(`{"type":"block","tag":"block"}`),
		},
		Route: routeCfg,
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

var outboundTypeAllowlist = map[string]bool{
	"vless":     true,
	"hysteria2": true,
	"tuic":      true,
	"direct":    true,
}

var dnsLabelPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)

func validateProfileForRender(prof profile.Profile) error {
	switch prof.Family {
	case profile.FamilyReality, profile.FamilyHysteria2, profile.FamilyTUIC:
	default:
		return fmt.Errorf("invalid profile for render: unsupported protocol")
	}
	if !isSafeHost(strings.TrimSpace(strings.ToLower(prof.Endpoint.Host))) {
		return fmt.Errorf("invalid profile for render: malformed endpoint host")
	}
	if prof.Endpoint.Port < 1 || prof.Endpoint.Port > 65535 {
		return fmt.Errorf("invalid profile for render: endpoint port out of range")
	}
	return nil
}

func validateRenderedOutbound(out []byte) (json.RawMessage, error) {
	var raw any
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("rendered outbound json invalid: %w", err)
	}
	obj, err := extractOutboundObject(raw)
	if err != nil {
		return nil, err
	}
	typ, _ := obj["type"].(string)
	typ = strings.TrimSpace(strings.ToLower(typ))
	if !outboundTypeAllowlist[typ] {
		return nil, fmt.Errorf("rendered outbound rejected: unknown outbound type")
	}
	obj["type"] = typ
	obj["tag"] = "proxy"

	if typ != "direct" {
		host, ok := obj["server"].(string)
		if !ok || !isSafeHost(strings.TrimSpace(strings.ToLower(host))) {
			return nil, fmt.Errorf("rendered outbound rejected: malformed server")
		}
		obj["server"] = strings.TrimSpace(strings.ToLower(host))

		port, err := toSafePort(obj["server_port"])
		if err != nil {
			return nil, fmt.Errorf("rendered outbound rejected: invalid server_port")
		}
		obj["server_port"] = port
	}

	switch typ {
	case "vless":
		if strings.TrimSpace(asString(obj["uuid"])) == "" {
			return nil, fmt.Errorf("rendered outbound rejected: missing vless uuid")
		}
		if strings.TrimSpace(asString(obj["flow"])) == "" {
			return nil, fmt.Errorf("rendered outbound rejected: missing vless flow")
		}
	case "hysteria2":
		if strings.TrimSpace(asString(obj["password"])) == "" {
			return nil, fmt.Errorf("rendered outbound rejected: missing hysteria2 password")
		}
	case "tuic":
		if strings.TrimSpace(asString(obj["uuid"])) == "" || strings.TrimSpace(asString(obj["password"])) == "" {
			return nil, fmt.Errorf("rendered outbound rejected: missing tuic credentials")
		}
	}

	clean, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("rendered outbound rejected: marshal failed")
	}
	return json.RawMessage(clean), nil
}

func extractOutboundObject(raw any) (map[string]any, error) {
	obj, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("rendered outbound rejected: root must be object")
	}
	if _, hasType := obj["type"]; hasType {
		return obj, nil
	}
	outbounds, ok := obj["outbounds"].([]any)
	if !ok || len(outbounds) != 1 {
		return nil, fmt.Errorf("rendered outbound rejected: expected a single outbound")
	}
	item, ok := outbounds[0].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("rendered outbound rejected: outbound must be object")
	}
	return item, nil
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}

func toSafePort(v any) (int, error) {
	switch t := v.(type) {
	case float64:
		p := int(t)
		if float64(p) != t {
			return 0, fmt.Errorf("non-integer port")
		}
		if p < 1 || p > 65535 {
			return 0, fmt.Errorf("port out of range")
		}
		return p, nil
	case int:
		if t < 1 || t > 65535 {
			return 0, fmt.Errorf("port out of range")
		}
		return t, nil
	default:
		return 0, fmt.Errorf("invalid port type")
	}
}

func isSafeHost(host string) bool {
	if host == "" || len(host) > 253 {
		return false
	}
	if ip, err := netip.ParseAddr(host); err == nil {
		return ip.IsValid()
	}
	parts := strings.Split(host, ".")
	for _, p := range parts {
		if !dnsLabelPattern.MatchString(p) {
			return false
		}
	}
	return true
}

func parseListenAddr(addr string) (string, int, error) {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return "", 0, fmt.Errorf("invalid probe listen addr %q (expected host:port): %w", addr, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		return "", 0, fmt.Errorf("invalid probe listen port %q in %q", portStr, addr)
	}
	if host == "" {
		host = "127.0.0.1"
	}
	return host, port, nil
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
