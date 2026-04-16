package main

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"filippo.io/age"
	"github.com/kaveh/shadownet-agent/pkg/importctl"
	"github.com/kaveh/shadownet-agent/pkg/orchestrator"
	"github.com/kaveh/shadownet-agent/pkg/policy"
	"github.com/kaveh/shadownet-agent/pkg/profile"
	"github.com/kaveh/shadownet-agent/pkg/report"
	"github.com/kaveh/shadownet-agent/pkg/sbctl"
)

func main() {
	if err := run(); err != nil {
		log.Printf("fatal: %v", err)
		os.Exit(1)
	}
}

type agentState struct {
	StateDir           string   `json:"state_dir"`
	ProfilesLoaded     int      `json:"profiles_loaded"`
	SelectedProfileID  string   `json:"selected_profile_id"`
	SelectionReason    string   `json:"selection_reason"`
	FallbackCandidates []string `json:"fallback_candidates"`
	ConfigPath         string   `json:"config_path"`
	SingBoxBinary      string   `json:"sing_box_binary"`
	SingBoxPID         int      `json:"sing_box_pid"`
	UpdatedAtUnix      int64    `json:"updated_at_unix"`
}

type options struct {
	StateDir             string
	ImportPath           string
	MasterKey            string
	TemplatesDir         string
	TrustedSignerPubsB64 string
	AgeIdentity          string
	SingBoxBin           string
	ReportOut            string
	RenderOnly           bool
	ValidateOnly         bool
	Verbose              bool
	UsePi                bool
	PiEndpoint           string
	PiTimeoutMS          int
	PiCmd                string
}

func run() error {
	var opts options
	flag.StringVar(&opts.StateDir, "state-dir", "", "State directory")
	flag.StringVar(&opts.ImportPath, "import", "", "Import a signed/encrypted bundle file from disk")
	flag.StringVar(&opts.MasterKey, "master-key", "", "Master key for local encrypted storage (32 raw bytes, 64 hex chars, or base64/base64url encoding of 32 bytes)")
	flag.StringVar(&opts.TemplatesDir, "templates-dir", "", "Directory containing outbound templates")
	flag.StringVar(&opts.TrustedSignerPubsB64, "trusted-signer-pub-b64url", "", "Trusted signer ed25519 public keys (base64url), comma-separated")
	flag.StringVar(&opts.AgeIdentity, "age-identity", "", "age X25519 identity for bundle decryption")
	flag.StringVar(&opts.SingBoxBin, "sing-box-bin", "", "Path to sing-box binary (optional; falls back to PATH)")
	flag.StringVar(&opts.ReportOut, "report-out", "", "Write a local report JSON file and exit (no sensitive identifiers)")
	flag.BoolVar(&opts.RenderOnly, "render-only", false, "Render config to disk and exit without starting sing-box")
	flag.BoolVar(&opts.ValidateOnly, "validate-only", false, "Validate rendered config with sing-box and exit")
	flag.BoolVar(&opts.Verbose, "verbose", false, "Enable verbose logging")
	flag.BoolVar(&opts.UsePi, "use-pi", false, "Enable optional Pi orchestrator (never blocks; falls back to deterministic policy)")
	flag.StringVar(&opts.PiEndpoint, "pi-endpoint", "", "Optional Pi TCP endpoint (host:port). If empty, uses a child process over stdin/stdout")
	flag.IntVar(&opts.PiTimeoutMS, "pi-timeout-ms", 1200, "Pi decision timeout in milliseconds")
	flag.StringVar(&opts.PiCmd, "pi-cmd", "pi", "Pi command to run when using stdin/stdout mode")
	flag.Parse()

	if opts.StateDir == "" {
		opts.StateDir = os.Getenv("SHADOWNET_STATE_DIR")
	}
	if opts.StateDir == "" {
		opts.StateDir = defaultStateDir()
	}
	if opts.MasterKey == "" {
		opts.MasterKey = os.Getenv("SHADOWNET_MASTER_KEY")
	}
	masterKey, err := profile.ParseMasterKey(opts.MasterKey)
	if err != nil {
		return fmt.Errorf("missing or invalid master key: %w (set --master-key or SHADOWNET_MASTER_KEY)", err)
	}
	if opts.TemplatesDir == "" {
		opts.TemplatesDir = os.Getenv("SHADOWNET_TEMPLATES_DIR")
	}
	if opts.TemplatesDir == "" {
		opts.TemplatesDir = filepath.Join(".", "templates")
	}
	if opts.TrustedSignerPubsB64 == "" {
		opts.TrustedSignerPubsB64 = os.Getenv("SHADOWNET_TRUSTED_SIGNER_PUB_B64URL")
	}
	if opts.AgeIdentity == "" {
		opts.AgeIdentity = os.Getenv("SHADOWNET_AGE_IDENTITY")
	}
	if opts.SingBoxBin == "" {
		opts.SingBoxBin = os.Getenv("SHADOWNET_SINGBOX_BIN")
	}

	if opts.Verbose {
		log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	} else {
		log.SetFlags(log.LstdFlags)
	}

	log.Printf("inside: state_dir=%s templates_dir=%s render_only=%v validate_only=%v", opts.StateDir, opts.TemplatesDir, opts.RenderOnly, opts.ValidateOnly)

	runtimeDir := filepath.Join(opts.StateDir, "runtime")
	keysDir := filepath.Join(opts.StateDir, "keys")
	if err := os.MkdirAll(runtimeDir, 0o700); err != nil {
		return err
	}
	_ = os.MkdirAll(keysDir, 0o700)

	storePath := filepath.Join(opts.StateDir, "profiles.enc")
	templatesPath := filepath.Join(opts.StateDir, "templates.enc")
	statePath := filepath.Join(opts.StateDir, "state.json")
	configPath := filepath.Join(runtimeDir, "config.json")

	store, err := profile.NewStore(storePath, masterKey)
	if err != nil {
		return err
	}
	templateStore, err := profile.NewTemplateStore(templatesPath, masterKey)
	if err != nil {
		return fmt.Errorf("failed to create template store: %w", err)
	}

	if opts.ReportOut != "" {
		r := report.Generate(opts.StateDir, masterKey)
		if err := report.WriteFile(opts.ReportOut, r); err != nil {
			return err
		}
		log.Printf("report written: %s", opts.ReportOut)
		return nil
	}

	if opts.ImportPath != "" {
		ageIdentity, err := loadAgeIdentity(opts.AgeIdentity, filepath.Join(keysDir, "age_identity.txt"))
		if err != nil {
			return err
		}
		trustedKeys, err := parseTrustedSignerKeys(opts.TrustedSignerPubsB64, filepath.Join(keysDir, "trusted_signers.txt"))
		if err != nil {
			return err
		}
		importer := importctl.NewImporterWithTemplates(store, templateStore, trustedKeys, ageIdentity)
		payload, err := importer.ImportFile(opts.ImportPath)
		if err != nil {
			return err
		}
		if err := importer.ProcessAndStore(payload); err != nil {
			return err
		}
		log.Printf("import ok: profiles=%d revocations=%d templates=%d", len(payload.Profiles), len(payload.Revocations), len(payload.Templates))
	}

	allProfiles, err := store.Load()
	if err != nil {
		if strings.Contains(err.Error(), "decryption failed (wrong key or corrupted data)") {
			return fmt.Errorf("failed to decrypt profiles store (wrong --master-key or corrupted file): %w (try removing %s to reset)", err, storePath)
		}
		return fmt.Errorf("failed to load profiles store: %w", err)
	}
	log.Printf("inside: profiles_loaded=%d", len(allProfiles))

	var candidates []profile.Profile
	for _, p := range allProfiles {
		if !p.Enabled {
			continue
		}
		if p.ManualDisabled {
			continue
		}
		candidates = append(candidates, p)
	}
	if len(candidates) == 0 {
		return fmt.Errorf("no enabled profiles available: import a bundle with --import")
	}

	engine := policy.Engine{MaxBurstFailures: 3}
	ranked := engine.RankProfiles(candidates)
	if len(ranked) == 0 {
		return fmt.Errorf("no viable profiles after cooldown filters")
	}

	selected := ranked[0]
	policyReason := fmt.Sprintf("score=%.1f ewma=%.2f cooldown_until=%d", selected.Health.Score, selected.Health.SuccessEWMA, selected.Health.CooldownUntil)
	policyConfidence := 1.0
	if len(ranked) > 1 {
		diff := ranked[0].Health.Score - ranked[1].Health.Score
		policyConfidence = diff / 20.0
		if policyConfidence < 0 {
			policyConfidence = 0
		}
		if policyConfidence > 1 {
			policyConfidence = 1
		}
	}
	if math.IsNaN(policyConfidence) {
		policyConfidence = 0
	}
	fallbackCandidates := make([]string, 0, 3)
	for i := 1; i < len(ranked) && len(fallbackCandidates) < 3; i++ {
		fallbackCandidates = append(fallbackCandidates, ranked[i].ID)
	}

	finalSelected := selected
	finalReason := policyReason
	finalConfidence := policyConfidence
	finalSource := "policy"

	shouldInvokePi := opts.UsePi || (policyConfidence < 0.6 && len(ranked) > 1)
	var piClient orchestrator.Client
	if shouldInvokePi {
		cfg := orchestrator.Config{
			UseTCP:   strings.TrimSpace(opts.PiEndpoint) != "",
			Endpoint: strings.TrimSpace(opts.PiEndpoint),
			Command:  strings.TrimSpace(opts.PiCmd),
			Timeout:  time.Duration(opts.PiTimeoutMS) * time.Millisecond,
		}
		c, err := orchestrator.NewClient(cfg)
		if err != nil {
			log.Printf("[orchestrator] unavailable: %v (fallback to policy)", err)
			shouldInvokePi = false
		} else {
			piClient = c
			defer func() { _ = piClient.Close() }()
		}
	}
	if piClient != nil {
		log.Printf("[policy] candidates=%v policy_confidence=%.2f", func() []string {
			out := make([]string, 0, len(ranked))
			for i := 0; i < len(ranked) && i < 5; i++ {
				out = append(out, ranked[i].ID)
			}
			return out
		}(), policyConfidence)
		log.Printf("[orchestrator] invoked")
		now := time.Now().Unix()
		history := orchestrator.DecisionHistory{RecentSwitches: nil, FailRate: 1 - selected.Health.SuccessEWMA}
		req := orchestrator.BuildRequest(now, ranked, history, orchestrator.GuardConfig{MaxSwitchRate: 3, NoUDP: false}, orchestrator.NetworkState{})
		res := orchestrator.DecideWithFallback(
			context.Background(),
			piClient,
			time.Duration(opts.PiTimeoutMS)*time.Millisecond,
			selected,
			ranked,
			req,
			selected,
			policyReason,
			policyConfidence,
		)
		finalSelected = res.SelectedProfile
		finalReason = fmt.Sprintf("%s source=%s confidence=%.2f", res.Reason, res.Source, res.Confidence)
		finalConfidence = res.Confidence
		finalSource = res.Source
		log.Printf("[orchestrator] decision=%s profile=%s confidence=%.2f reason=%q", res.Source, res.SelectedProfile.ID, res.Confidence, res.Reason)
	}

	log.Printf("select ok: profile=%s source=%s confidence=%.2f reason=%s fallbacks=%v", finalSelected.ID, finalSource, finalConfidence, finalReason, fallbackCandidates)

	templateText, err := resolveTemplateText(finalSelected, templateStore, opts.TemplatesDir)
	if err != nil {
		return err
	}

	rendered, err := sbctl.RenderConfigToFile(finalSelected, templateText, configPath)
	if err != nil {
		return err
	}
	log.Printf("render ok: config=%s profile=%s family=%s", rendered.ConfigPath, finalSelected.ID, finalSelected.Family)

	ctrl := sbctl.NewController(runtimeDir, opts.SingBoxBin)
	if opts.RenderOnly {
		return writeState(statePath, agentState{
			StateDir:           opts.StateDir,
			ProfilesLoaded:     len(allProfiles),
			SelectedProfileID:  finalSelected.ID,
			SelectionReason:    finalReason,
			FallbackCandidates: fallbackCandidates,
			ConfigPath:         rendered.ConfigPath,
			SingBoxBinary:      ctrl.BinaryPath,
			SingBoxPID:         0,
			UpdatedAtUnix:      time.Now().Unix(),
		})
	}

	mkState := func(pid int) agentState {
		return agentState{
			StateDir:           opts.StateDir,
			ProfilesLoaded:     len(allProfiles),
			SelectedProfileID:  finalSelected.ID,
			SelectionReason:    finalReason,
			FallbackCandidates: fallbackCandidates,
			ConfigPath:         rendered.ConfigPath,
			SingBoxBinary:      ctrl.BinaryPath,
			SingBoxPID:         pid,
			UpdatedAtUnix:      time.Now().Unix(),
		}
	}

	if opts.ValidateOnly {
		if err := ctrl.ValidateConfig(rendered.ConfigPath); err != nil {
			if errors.Is(err, sbctl.ErrBinaryNotFound) {
				_ = writeState(statePath, mkState(0))
				return fmt.Errorf("%w: set --sing-box-bin or install sing-box (config rendered at %s)", err, rendered.ConfigPath)
			}
			return err
		}
		return writeState(statePath, mkState(0))
	}

	if err := ctrl.ApplyAndReload(string(rendered.ConfigBytes)); err != nil {
		if errors.Is(err, sbctl.ErrBinaryNotFound) {
			_ = writeState(statePath, mkState(0))
			return fmt.Errorf("%w: set --sing-box-bin or install sing-box (config rendered at %s)", err, rendered.ConfigPath)
		}
		return err
	}

	return writeState(statePath, mkState(ctrl.PID()))
}

func defaultStateDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".", ".shadownet")
	}
	if runtime.GOOS == "linux" {
		if xdg := os.Getenv("XDG_STATE_HOME"); xdg != "" {
			return filepath.Join(xdg, "shadownet")
		}
		return filepath.Join(home, ".local", "state", "shadownet")
	}
	return filepath.Join(home, ".shadownet")
}

func parseTrustedSignerKeys(keysCSV string, fallbackPath string) ([]ed25519.PublicKey, error) {
	if keysCSV == "" {
		b, err := os.ReadFile(fallbackPath)
		if err == nil {
			keysCSV = string(b)
		}
	}
	keysCSV = strings.TrimSpace(keysCSV)
	if keysCSV == "" {
		return nil, fmt.Errorf("missing trusted signer keys: set --trusted-signer-pub-b64url or SHADOWNET_TRUSTED_SIGNER_PUB_B64URL")
	}

	parts := strings.FieldsFunc(keysCSV, func(r rune) bool { return r == ',' || r == '\n' || r == '\r' || r == '\t' || r == ' ' })
	var out []ed25519.PublicKey
	for _, p := range parts {
		pubBytes, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(p))
		if err != nil {
			return nil, fmt.Errorf("invalid signer public key (base64url): %w", err)
		}
		if len(pubBytes) != ed25519.PublicKeySize {
			return nil, fmt.Errorf("invalid signer public key size: %d", len(pubBytes))
		}
		out = append(out, ed25519.PublicKey(pubBytes))
	}
	return out, nil
}

func loadAgeIdentity(value string, fallbackPath string) (*age.X25519Identity, error) {
	if strings.TrimSpace(value) == "" {
		if b, err := os.ReadFile(fallbackPath); err == nil {
			value = string(b)
		}
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, fmt.Errorf("missing age identity: set --age-identity or SHADOWNET_AGE_IDENTITY")
	}
	ageIdentity, err := age.ParseX25519Identity(value)
	if err != nil {
		return nil, fmt.Errorf("invalid age identity: %w", err)
	}
	return ageIdentity, nil
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
		return "", fmt.Errorf("missing outbound template for profile %q (looked for %s): %w", p.ID, full, err)
	}
	return string(b), nil
}

func writeState(path string, st agentState) error {
	raw, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
