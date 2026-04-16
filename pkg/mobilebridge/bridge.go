package mobilebridge

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"filippo.io/age"
	"github.com/kaveh/shadownet-agent/pkg/importctl"
	"github.com/kaveh/shadownet-agent/pkg/orchestrator"
	"github.com/kaveh/shadownet-agent/pkg/policy"
	"github.com/kaveh/shadownet-agent/pkg/profile"
	"github.com/kaveh/shadownet-agent/pkg/sbctl"
)

type AgentConfig struct {
	StateDir             string `json:"state_dir"`
	MasterKey            string `json:"master_key"`
	TemplatesDir         string `json:"templates_dir"`
	TrustedSignerPubsB64 string `json:"trusted_signer_pub_b64url"`
	AgeIdentity          string `json:"age_identity"`
	PollIntervalSec      int    `json:"poll_interval_sec"`
	ConfigPath           string `json:"config_path"`
	UsePi                bool   `json:"use_pi"`
	PiEndpoint           string `json:"pi_endpoint"`
	PiCommand            string `json:"pi_command"`
	PiTimeoutMS          int    `json:"pi_timeout_ms"`
}

type AgentStatus struct {
	Running        bool   `json:"running"`
	CurrentProfile string `json:"current_profile"`
	LastAction     string `json:"last_action"`
	LastError      string `json:"last_error,omitempty"`
	UpdatedAtUnix  int64  `json:"updated_at_unix"`
}

type runtimeState struct {
	mu      sync.Mutex
	cfg     AgentConfig
	status  AgentStatus
	stopCh  chan struct{}
	doneCh  chan struct{}
	pi      orchestrator.Client
	history []string
}

var state = &runtimeState{}

func StartAgent(config string) {
	var cfg AgentConfig
	if err := json.Unmarshal([]byte(config), &cfg); err != nil {
		setError(fmt.Sprintf("invalid config: %v", err))
		return
	}
	if err := validateConfig(&cfg); err != nil {
		setError(err.Error())
		return
	}

	state.mu.Lock()
	oldStop := state.stopCh
	oldDone := state.doneCh
	oldPi := state.pi
	state.stopCh = nil
	state.doneCh = nil
	state.pi = nil
	state.mu.Unlock()

	if oldStop != nil {
		close(oldStop)
	}
	if oldDone != nil {
		<-oldDone
	}
	if oldPi != nil {
		_ = oldPi.Close()
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	state.cfg = cfg
	state.status = AgentStatus{
		Running:       true,
		LastAction:    "starting",
		UpdatedAtUnix: time.Now().Unix(),
	}
	state.stopCh = make(chan struct{})
	state.doneCh = make(chan struct{})
	state.history = nil

	go runLoop(cfg, state.stopCh, state.doneCh)
}

func StopAgent() {
	state.mu.Lock()
	stopCh := state.stopCh
	doneCh := state.doneCh
	pi := state.pi
	state.stopCh = nil
	state.doneCh = nil
	state.pi = nil
	state.status.Running = false
	state.status.LastAction = "stopped"
	state.status.UpdatedAtUnix = time.Now().Unix()
	state.mu.Unlock()

	if stopCh != nil {
		close(stopCh)
	}
	if doneCh != nil {
		<-doneCh
	}
	if pi != nil {
		_ = pi.Close()
	}
}

func ImportBundle(path string) error {
	state.mu.Lock()
	cfg := state.cfg
	state.mu.Unlock()
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("missing bundle path")
	}
	if err := validateConfig(&cfg); err != nil {
		return err
	}
	masterKey, err := profile.ParseMasterKey(cfg.MasterKey)
	if err != nil {
		return fmt.Errorf("invalid master_key: %w", err)
	}
	storePath := filepath.Join(cfg.StateDir, "profiles.enc")
	templatesPath := filepath.Join(cfg.StateDir, "templates.enc")
	store, err := profile.NewStore(storePath, masterKey)
	if err != nil {
		return err
	}
	templateStore, err := profile.NewTemplateStore(templatesPath, masterKey)
	if err != nil {
		return err
	}
	trustedKeys, err := parseTrustedSignerKeys(cfg.TrustedSignerPubsB64)
	if err != nil {
		return err
	}
	ageIdentity, err := age.ParseX25519Identity(strings.TrimSpace(cfg.AgeIdentity))
	if err != nil {
		return fmt.Errorf("invalid age identity: %w", err)
	}

	importer := importctl.NewImporterWithTemplates(store, templateStore, trustedKeys, ageIdentity)
	payload, err := importer.ImportFile(path)
	if err != nil {
		return err
	}
	return importer.ProcessAndStore(payload)
}

func GetStatus() string {
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.status.UpdatedAtUnix == 0 {
		state.status.UpdatedAtUnix = time.Now().Unix()
	}
	raw, _ := json.Marshal(state.status)
	return string(raw)
}

func runLoop(cfg AgentConfig, stopCh <-chan struct{}, doneCh chan<- struct{}) {
	defer close(doneCh)

	if cfg.UsePi {
		piCfg := orchestrator.Config{
			UseTCP:   strings.TrimSpace(cfg.PiEndpoint) != "",
			Endpoint: strings.TrimSpace(cfg.PiEndpoint),
			Command:  strings.TrimSpace(cfg.PiCommand),
			Timeout:  time.Duration(cfg.PiTimeoutMS) * time.Millisecond,
		}
		pi, err := orchestrator.NewClient(piCfg)
		if err == nil {
			state.mu.Lock()
			state.pi = pi
			state.mu.Unlock()
		} else {
			updateError(fmt.Sprintf("orchestrator unavailable: %v", err))
		}
	}

	interval := time.Duration(cfg.PollIntervalSec) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	runOnce(cfg)
	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			runOnce(cfg)
		}
	}
}

func runOnce(cfg AgentConfig) {
	masterKey, err := profile.ParseMasterKey(cfg.MasterKey)
	if err != nil {
		updateError(fmt.Sprintf("invalid master_key: %v", err))
		return
	}
	storePath := filepath.Join(cfg.StateDir, "profiles.enc")
	templatesPath := filepath.Join(cfg.StateDir, "templates.enc")
	store, err := profile.NewStore(storePath, masterKey)
	if err != nil {
		updateError(fmt.Sprintf("store error: %v", err))
		return
	}
	templateStore, err := profile.NewTemplateStore(templatesPath, masterKey)
	if err != nil {
		updateError(fmt.Sprintf("template store error: %v", err))
		return
	}
	profiles, err := store.Load()
	if err != nil {
		updateError(fmt.Sprintf("load profiles: %v", err))
		return
	}

	var candidates []profile.Profile
	for _, p := range profiles {
		if p.Enabled && !p.ManualDisabled {
			candidates = append(candidates, p)
		}
	}
	if len(candidates) == 0 {
		updateError("no profiles available")
		return
	}

	engine := policy.Engine{MaxBurstFailures: 3}
	ranked := engine.RankProfiles(candidates)
	if len(ranked) == 0 {
		updateError("no viable profiles after cooldown filters")
		return
	}

	selected := ranked[0]
	reason := fmt.Sprintf("policy score=%.1f", selected.Health.Score)

	state.mu.Lock()
	pi := state.pi
	recent := append([]string(nil), state.history...)
	state.mu.Unlock()

	if pi != nil && len(ranked) > 1 {
		req := orchestrator.BuildRequest(
			time.Now().Unix(),
			ranked,
			orchestrator.DecisionHistory{RecentSwitches: recent, FailRate: 1 - selected.Health.SuccessEWMA},
			orchestrator.GuardConfig{MaxSwitchRate: 3, NoUDP: false},
			orchestrator.NetworkState{},
		)
		res := orchestrator.DecideWithFallback(
			context.Background(),
			pi,
			time.Duration(cfg.PiTimeoutMS)*time.Millisecond,
			selected,
			ranked,
			req,
			selected,
			reason,
			0.5,
		)
		selected = res.SelectedProfile
		reason = fmt.Sprintf("%s source=%s", res.Reason, res.Source)
	}

	templateText, err := resolveTemplateText(selected, templateStore, cfg.TemplatesDir)
	if err != nil {
		updateError(err.Error())
		return
	}

	configPath := cfg.ConfigPath
	if strings.TrimSpace(configPath) == "" {
		configPath = filepath.Join(cfg.StateDir, "runtime", "config.json")
	}
	if _, err := sbctl.RenderConfigToFile(selected, templateText, configPath); err != nil {
		updateError(fmt.Sprintf("render config: %v", err))
		return
	}

	state.mu.Lock()
	state.history = append(state.history, selected.ID)
	if len(state.history) > 8 {
		state.history = state.history[len(state.history)-8:]
	}
	state.status = AgentStatus{
		Running:        true,
		CurrentProfile: selected.ID,
		LastAction:     reason,
		LastError:      "",
		UpdatedAtUnix:  time.Now().Unix(),
	}
	state.mu.Unlock()
}

func resolveTemplateText(p profile.Profile, store *profile.TemplateStore, templatesDir string) (string, error) {
	templates, err := store.Load()
	if err == nil {
		if p.TemplateRef != "" {
			if t, ok := templates[p.TemplateRef]; ok && strings.TrimSpace(t) != "" {
				return t, nil
			}
		}
		if t, ok := templates[string(p.Family)]; ok && strings.TrimSpace(t) != "" {
			return t, nil
		}
	}
	name := string(p.Family) + ".json"
	if p.TemplateRef != "" {
		name = p.TemplateRef
		if !strings.HasSuffix(strings.ToLower(name), ".json") {
			name += ".json"
		}
	}
	full := filepath.Join(templatesDir, filepath.Clean(name))
	b, err := os.ReadFile(full)
	if err != nil {
		return "", fmt.Errorf("missing template for %q (%s): %w", p.ID, full, err)
	}
	return string(b), nil
}

func parseTrustedSignerKeys(keysCSV string) ([]ed25519.PublicKey, error) {
	keysCSV = strings.TrimSpace(keysCSV)
	if keysCSV == "" {
		return nil, fmt.Errorf("missing trusted signer keys")
	}
	parts := strings.FieldsFunc(keysCSV, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r' || r == '\t' || r == ' '
	})
	var out []ed25519.PublicKey
	for _, p := range parts {
		pubBytes, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(p))
		if err != nil {
			return nil, fmt.Errorf("invalid signer public key: %w", err)
		}
		if len(pubBytes) != ed25519.PublicKeySize {
			return nil, fmt.Errorf("invalid signer public key size")
		}
		out = append(out, ed25519.PublicKey(pubBytes))
	}
	return out, nil
}

func validateConfig(cfg *AgentConfig) error {
	if strings.TrimSpace(cfg.StateDir) == "" {
		return fmt.Errorf("missing state_dir")
	}
	if _, err := profile.ParseMasterKey(cfg.MasterKey); err != nil {
		return fmt.Errorf("invalid master_key: %w", err)
	}
	if strings.TrimSpace(cfg.TemplatesDir) == "" {
		return fmt.Errorf("missing templates_dir")
	}
	if cfg.PollIntervalSec <= 0 {
		cfg.PollIntervalSec = 20
	}
	if cfg.PollIntervalSec < 10 {
		cfg.PollIntervalSec = 10
	}
	if cfg.PollIntervalSec > 300 {
		cfg.PollIntervalSec = 300
	}
	if cfg.PiTimeoutMS <= 0 {
		cfg.PiTimeoutMS = 1200
	}
	if strings.TrimSpace(cfg.PiCommand) == "" {
		cfg.PiCommand = "pi"
	}
	return nil
}

func setError(msg string) {
	state.mu.Lock()
	defer state.mu.Unlock()
	state.status = AgentStatus{
		Running:       false,
		LastAction:    "error",
		LastError:     msg,
		UpdatedAtUnix: time.Now().Unix(),
	}
}

func updateError(msg string) {
	state.mu.Lock()
	defer state.mu.Unlock()
	state.status.Running = true
	state.status.LastAction = "error"
	state.status.LastError = msg
	state.status.UpdatedAtUnix = time.Now().Unix()
}
