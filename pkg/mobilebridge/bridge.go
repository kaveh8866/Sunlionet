package mobilebridge

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"filippo.io/age"
	"github.com/kaveh/sunlionet-agent/pkg/assistant"
	"github.com/kaveh/sunlionet-agent/pkg/importctl"
	"github.com/kaveh/sunlionet-agent/pkg/orchestrator"
	"github.com/kaveh/sunlionet-agent/pkg/policy"
	"github.com/kaveh/sunlionet-agent/pkg/profile"
	"github.com/kaveh/sunlionet-agent/pkg/sbctl"
	"github.com/kaveh/sunlionet-agent/pkg/telemetry"
)

type AgentConfig struct {
	StateDir              string `json:"state_dir"`
	MasterKey             string `json:"master_key"`
	TemplatesDir          string `json:"templates_dir"`
	TrustedSignerPubsB64  string `json:"trusted_signer_pub_b64url"`
	AgeIdentity           string `json:"age_identity"`
	PollIntervalSec       int    `json:"poll_interval_sec"`
	ConfigPath            string `json:"config_path"`
	UsePi                 bool   `json:"use_pi"`
	PiEndpoint            string `json:"pi_endpoint"`
	PiCommand             string `json:"pi_command"`
	PiTimeoutMS           int    `json:"pi_timeout_ms"`
	AdaptiveMode          bool   `json:"adaptive_mode"`
	TelemetryEnabled      bool   `json:"telemetry_enabled"`
	TelemetryCollectorKey string `json:"telemetry_collector_pub_b64url"`
	TelemetryEndpoint     string `json:"telemetry_endpoint"`
	TelemetryTransport    string `json:"telemetry_transport"`
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
	var raw map[string]json.RawMessage
	_ = json.Unmarshal([]byte(config), &raw)
	if err := json.Unmarshal([]byte(config), &cfg); err != nil {
		setError(fmt.Sprintf("invalid config: %v", err))
		return
	}
	if _, ok := raw["adaptive_mode"]; !ok {
		cfg.AdaptiveMode = true
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
	replayPath := filepath.Join(cfg.StateDir, "replay.enc")
	trustPath := filepath.Join(cfg.StateDir, "trust.enc")
	store, err := profile.NewStore(storePath, masterKey)
	if err != nil {
		return err
	}
	templateStore, err := profile.NewTemplateStore(templatesPath, masterKey)
	if err != nil {
		return err
	}
	replayStore, err := importctl.NewReplayStore(replayPath, masterKey)
	if err != nil {
		return err
	}
	var trustState *importctl.TrustState
	trustStore, err := importctl.NewTrustStore(trustPath, masterKey)
	if err != nil {
		return err
	}
	if loadedTrust, err := trustStore.Load(); err == nil && loadedTrust.StateHash != "" {
		trustState = &loadedTrust
	}
	trustedKeys, err := parseTrustedSignerKeys(cfg.TrustedSignerPubsB64)
	if err != nil {
		return err
	}
	ageIdentity, err := age.ParseX25519Identity(strings.TrimSpace(cfg.AgeIdentity))
	if err != nil {
		return fmt.Errorf("invalid age identity: %w", err)
	}

	importer := importctl.NewImporterWithTrust(store, templateStore, replayStore, trustedKeys, ageIdentity, trustState)
	payload, err := importer.ImportFile(path)
	if err != nil {
		recordTelemetry(cfg, telemetryCodeForImportError(err))
		return err
	}
	if err := importer.ProcessAndStore(payload); err != nil {
		recordTelemetry(cfg, telemetry.EventConfigInvalid)
		return err
	}
	return nil
}

func ResetLearning() error {
	state.mu.Lock()
	cfg := state.cfg
	state.mu.Unlock()
	if err := validateConfig(&cfg); err != nil {
		return err
	}
	masterKey, err := profile.ParseMasterKey(cfg.MasterKey)
	if err != nil {
		return fmt.Errorf("invalid master_key: %w", err)
	}
	adaptiveStore, err := policy.NewAdaptiveStore(filepath.Join(cfg.StateDir, "adaptive.enc"), masterKey)
	if err != nil {
		return err
	}
	return adaptiveStore.Reset()
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
			recordTelemetry(cfg, telemetry.EventCoreStartFailure)
		}
	}

	interval := time.Duration(cfg.PollIntervalSec) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	if err := runOnce(cfg); err != nil && isFatalRuntimeError(err) {
		return
	}
	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			flushTelemetry(cfg)
			if err := runOnce(cfg); err != nil && isFatalRuntimeError(err) {
				return
			}
		}
	}
}

func runOnce(cfg AgentConfig) error {
	masterKey, err := profile.ParseMasterKey(cfg.MasterKey)
	if err != nil {
		updateError(fmt.Sprintf("invalid master_key: %v", err))
		recordTelemetry(cfg, telemetry.EventConfigInvalid)
		return err
	}
	storePath := filepath.Join(cfg.StateDir, "profiles.enc")
	templatesPath := filepath.Join(cfg.StateDir, "templates.enc")
	store, err := profile.NewStore(storePath, masterKey)
	if err != nil {
		updateError(fmt.Sprintf("store error: %v", err))
		recordTelemetry(cfg, telemetry.EventConfigInvalid)
		return err
	}
	templateStore, err := profile.NewTemplateStore(templatesPath, masterKey)
	if err != nil {
		updateError(fmt.Sprintf("template store error: %v", err))
		recordTelemetry(cfg, telemetry.EventConfigInvalid)
		return err
	}
	profiles, err := store.Load()
	if err != nil {
		if errors.Is(err, profile.ErrDecryptionFailed) || errors.Is(err, profile.ErrCorruptStore) {
			setError("corrupted encrypted state (profiles). reset state_dir to recover")
			return err
		}
		updateError(fmt.Sprintf("load profiles: %v", err))
		return err
	}

	var candidates []profile.Profile
	for _, p := range profiles {
		if p.Enabled && !p.ManualDisabled {
			candidates = append(candidates, p)
		}
	}
	if len(candidates) == 0 {
		updateError("no profiles available")
		recordTelemetry(cfg, telemetry.EventConfigInvalid)
		return errors.New("no profiles available")
	}

	engine := policy.Engine{MaxBurstFailures: 3}
	adaptivePath := filepath.Join(cfg.StateDir, "adaptive.enc")
	adaptiveStore, err := policy.NewAdaptiveStore(adaptivePath, masterKey)
	if err != nil {
		updateError(fmt.Sprintf("adaptive store: %v", err))
		return err
	}
	adaptiveState, err := adaptiveStore.Load()
	if err != nil {
		if errors.Is(err, policy.ErrDecryptionFailedAdaptive) || errors.Is(err, policy.ErrCorruptAdaptiveStore) {
			// If adaptive state is corrupt, we can just reset it instead of failing everything
			adaptiveState = policy.NewAdaptiveState(80)
			_ = adaptiveStore.Save(adaptiveState)
		} else {
			updateError(fmt.Sprintf("adaptive state: %v", err))
			recordTelemetry(cfg, telemetry.EventConfigInvalid)
			return err
		}
	}
	adaptiveState.SetEnabled(cfg.AdaptiveMode)
	engine.AdaptiveState = adaptiveState
	selected, decision, ranked := engine.SelectProfile(candidates)
	if len(ranked) == 0 {
		updateError("no viable profiles after cooldown filters")
		recordTelemetry(cfg, telemetry.EventProxyHandshakeTimeout)
		return errors.New("no viable profiles after cooldown filters")
	}
	reason := decision.Reason
	if strings.TrimSpace(reason) == "" {
		reason = fmt.Sprintf("policy score=%.1f", selected.Health.Score)
	}

	state.mu.Lock()
	pi := state.pi
	recent := append([]string(nil), state.history...)
	state.mu.Unlock()

	if pi != nil && len(ranked) > 1 && decision.UseOrchestrator {
		hints := make([]orchestrator.ScoreHint, 0, len(decision.Scores))
		for _, s := range decision.Scores {
			hints = append(hints, orchestrator.ScoreHint{ProfileID: s.ProfileID, Score: s.Score})
		}
		req := orchestrator.BuildRequest(
			time.Now().Unix(),
			ranked,
			orchestrator.DecisionHistory{RecentSwitches: recent, FailRate: 1 - selected.Health.SuccessEWMA},
			orchestrator.GuardConfig{MaxSwitchRate: 3, NoUDP: false},
			orchestrator.NetworkState{},
		)
		req.Adaptive = orchestrator.AdaptiveInput{
			Scores:         hints,
			RecentFailures: decision.RecentFailures,
		}
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
	known := make(map[string]struct{}, len(ranked))
	for _, p := range ranked {
		known[p.ID] = struct{}{}
	}
	if err := validateAction("switch_profile", selected.ID, known, len(recent)+1, 6); err != nil {
		updateError(fmt.Sprintf("action rejected: %v", err))
		adaptiveState.RecordAttempt(selected.ID, policy.AttemptSignal{ConnectOK: false, DNSOK: true, TCPHandshake: false, TLSSuccess: false}, "UNKNOWN", time.Now())
		_ = adaptiveStore.Save(adaptiveState)
		recordTelemetry(cfg, telemetry.EventUnknown)
		return err
	}

	templateText, err := resolveTemplateText(selected, templateStore, cfg.TemplatesDir)
	if err != nil {
		updateError(err.Error())
		adaptiveState.RecordAttempt(selected.ID, policy.AttemptSignal{ConnectOK: false, DNSOK: true, TCPHandshake: false, TLSSuccess: false}, "CONFIG_ERROR", time.Now())
		_ = adaptiveStore.Save(adaptiveState)
		recordTelemetry(cfg, telemetry.EventConfigInvalid)
		return err
	}

	configPath := cfg.ConfigPath
	if strings.TrimSpace(configPath) == "" {
		configPath = filepath.Join(cfg.StateDir, "runtime", "config.json")
	}
	if _, err := sbctl.RenderConfigToFile(selected, templateText, configPath); err != nil {
		updateError(fmt.Sprintf("render config: %v", err))
		adaptiveState.RecordAttempt(selected.ID, policy.AttemptSignal{ConnectOK: false, DNSOK: true, TCPHandshake: false, TLSSuccess: false}, "CONFIG_ERROR", time.Now())
		_ = adaptiveStore.Save(adaptiveState)
		recordTelemetry(cfg, telemetry.EventConfigInvalid)
		return err
	}
	adaptiveState.RecordSelection(selected.ID)
	adaptiveState.RecordAttempt(selected.ID, policy.AttemptSignal{
		LatencyMS:    0,
		ConnectOK:    true,
		DNSOK:        true,
		TCPHandshake: true,
		TLSSuccess:   true,
	}, "", time.Now())
	_ = adaptiveStore.Save(adaptiveState)

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
	return nil
}

func validateAction(action string, profileID string, known map[string]struct{}, switchCount int, maxPerMinute int) error {
	if strings.TrimSpace(action) == "" {
		return fmt.Errorf("missing action")
	}
	if _, ok := known[profileID]; !ok {
		return fmt.Errorf("unknown profile")
	}
	if maxPerMinute > 0 && switchCount > maxPerMinute {
		return fmt.Errorf("rate limit exceeded")
	}
	return nil
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
	msg = sanitizeStatusText(msg)
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
	msg = sanitizeStatusText(msg)
	state.mu.Lock()
	defer state.mu.Unlock()
	state.status.Running = true
	state.status.LastAction = "error"
	state.status.LastError = msg
	state.status.UpdatedAtUnix = time.Now().Unix()
}

func sanitizeStatusText(s string) string {
	out := assistant.RedactText(s, assistant.RedactionStrict)
	if len(out) > 240 {
		out = out[:240]
	}
	return strings.TrimSpace(out)
}

func isFatalRuntimeError(err error) bool {
	return errors.Is(err, profile.ErrDecryptionFailed) ||
		errors.Is(err, profile.ErrCorruptStore) ||
		errors.Is(err, policy.ErrDecryptionFailedAdaptive) ||
		errors.Is(err, policy.ErrCorruptAdaptiveStore)
}

func recordTelemetry(cfg AgentConfig, code telemetry.EventCode) {
	engine, err := newTelemetryEngine(cfg)
	if err != nil || !engine.Enabled() {
		return
	}
	_ = engine.Record(telemetry.DiagnosticEvent{
		Code:        code,
		CoreVersion: telemetry.CoreVersionSunLionetV1,
		Carrier:     telemetry.CarrierUnknown,
	})
}

func flushTelemetry(cfg AgentConfig) {
	engine, err := newTelemetryEngine(cfg)
	if err != nil || !engine.Enabled() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, _ = engine.FlushDue(ctx)
}

func newTelemetryEngine(cfg AgentConfig) (*telemetry.Engine, error) {
	if !cfg.TelemetryEnabled {
		return telemetry.NewEngine(telemetry.Config{Enabled: false}, nil)
	}
	tcfg := telemetry.Config{
		Enabled:                  true,
		QueuePath:                filepath.Join(cfg.StateDir, "telemetry.queue"),
		CollectorPublicKeyB64URL: strings.TrimSpace(cfg.TelemetryCollectorKey),
		Transport:                parseTelemetryTransport(cfg.TelemetryTransport),
		EndpointURL:              strings.TrimSpace(cfg.TelemetryEndpoint),
	}
	var sender telemetry.Sender
	if tcfg.EndpointURL != "" {
		sender = telemetry.HTTPSender{Endpoint: tcfg.EndpointURL}
	}
	return telemetry.NewEngine(tcfg, sender)
}

func parseTelemetryTransport(s string) telemetry.TransportKind {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "onion", "tor":
		return telemetry.TransportOnion
	case "i2p":
		return telemetry.TransportI2P
	case "mixnet", "mix":
		return telemetry.TransportMixnet
	case "domain_fronted", "fronted":
		return telemetry.TransportDomainFronted
	default:
		return telemetry.TransportUnknown
	}
}

func telemetryCodeForImportError(err error) telemetry.EventCode {
	if err == nil {
		return telemetry.EventUnknown
	}
	var importErr *importctl.ImportError
	if errors.As(err, &importErr) {
		switch importErr.Code {
		case importctl.CodeInvalidSignature:
			return telemetry.EventImportSignatureInvalid
		case importctl.CodeReplayDetected:
			return telemetry.EventImportReplayDetected
		case importctl.CodeMalformedBundle, importctl.CodeInvalidPayload, importctl.CodeCipherNotAllowed:
			return telemetry.EventConfigInvalid
		default:
			return telemetry.EventUnknown
		}
	}
	return telemetry.EventUnknown
}
