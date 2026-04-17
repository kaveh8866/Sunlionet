package main

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"filippo.io/age"
	"github.com/kaveh/shadownet-agent/pkg/detector"
	detreal "github.com/kaveh/shadownet-agent/pkg/detector/real"
	detsim "github.com/kaveh/shadownet-agent/pkg/detector/sim"
	"github.com/kaveh/shadownet-agent/pkg/e2e"
	"github.com/kaveh/shadownet-agent/pkg/importctl"
	"github.com/kaveh/shadownet-agent/pkg/mesh"
	meshreal "github.com/kaveh/shadownet-agent/pkg/mesh/real"
	meshsim "github.com/kaveh/shadownet-agent/pkg/mesh/sim"
	"github.com/kaveh/shadownet-agent/pkg/orchestrator"
	"github.com/kaveh/shadownet-agent/pkg/policy"
	"github.com/kaveh/shadownet-agent/pkg/profile"
	"github.com/kaveh/shadownet-agent/pkg/report"
	"github.com/kaveh/shadownet-agent/pkg/runtimecfg"
	"github.com/kaveh/shadownet-agent/pkg/sbctl"
	"github.com/kaveh/shadownet-agent/pkg/signalrx"
	signalreal "github.com/kaveh/shadownet-agent/pkg/signalrx/real"
	signalsim "github.com/kaveh/shadownet-agent/pkg/signalrx/sim"
)

var version = "dev"

type userError struct {
	Message string
	Err     error
}

func (e userError) Error() string {
	if e.Err == nil {
		return e.Message
	}
	return e.Message + ": " + e.Err.Error()
}

func (e userError) Unwrap() error { return e.Err }

func main() {
	if err := run(); err != nil {
		var ue userError
		if errors.As(err, &ue) {
			fmt.Fprintln(os.Stderr, "[Error]", ue.Message)
			if ue.Err != nil {
				fmt.Fprintln(os.Stderr, "→ Details:", sanitizeLogText(ue.Err.Error()))
			}
			os.Exit(1)
		}
		log.Printf("[fatal] %v", err)
		os.Exit(1)
	}
}

type agentState struct {
	StateDir           string          `json:"state_dir"`
	ProfilesLoaded     int             `json:"profiles_loaded"`
	SelectedProfileID  string          `json:"selected_profile_id"`
	SelectionReason    string          `json:"selection_reason"`
	FallbackCandidates []string        `json:"fallback_candidates"`
	ConfigPath         string          `json:"config_path"`
	SingBoxBinary      string          `json:"sing_box_binary"`
	SingBoxPID         int             `json:"sing_box_pid"`
	Status             string          `json:"status"`
	Probe              e2e.ProbeResult `json:"probe"`
	Attempts           []attemptState  `json:"attempts,omitempty"`
	UpdatedAtUnix      int64           `json:"updated_at_unix"`
}

type attemptState struct {
	Attempt    int             `json:"attempt"`
	ProfileID  string          `json:"profile_id"`
	Family     string          `json:"family"`
	ConfigPath string          `json:"config_path"`
	SingBoxPID int             `json:"sing_box_pid"`
	Probe      e2e.ProbeResult `json:"probe"`
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
	Mode                 string
	ProbeURL             string
	ProbeProxyAddr       string
	ProbeTimeoutMS       int
	MaxAttempts          int
	RuntimeAPIAddr       string
	RuntimeAPIKeepAlive  bool
}

func run() error {
	fmt.Printf("ShadowNet Inside %s\n", version)

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
	flag.StringVar(&opts.Mode, "mode", string(runtimecfg.ModeReal), "Runtime mode: real or simulation")
	flag.StringVar(&opts.ProbeURL, "probe-url", "", "If set, run an HTTP probe through sing-box and require success before accepting a profile (e.g. https://example.com)")
	flag.StringVar(&opts.ProbeProxyAddr, "probe-proxy-addr", "127.0.0.1:18080", "Local sing-box proxy listen address for probes (host:port)")
	flag.IntVar(&opts.ProbeTimeoutMS, "probe-timeout-ms", 10_000, "Probe timeout in milliseconds")
	flag.IntVar(&opts.MaxAttempts, "max-attempts", 3, "Maximum profiles to try before failing")
	flag.StringVar(&opts.RuntimeAPIAddr, "runtime-api-addr", "", "Optional local runtime API listen address (localhost only), e.g. 127.0.0.1:8080")
	flag.BoolVar(&opts.RuntimeAPIKeepAlive, "runtime-api-keepalive", false, "If set, keep the agent process running to serve the runtime API until interrupted")
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
	if opts.RuntimeAPIAddr == "" {
		opts.RuntimeAPIAddr = os.Getenv("SHADOWNET_RUNTIME_API_ADDR")
	}

	if opts.Verbose {
		log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	} else {
		log.SetFlags(log.LstdFlags)
	}
	log.SetPrefix("[ShadowNet Inside] ")

	log.Printf("state_dir=%s templates_dir=%s render_only=%v validate_only=%v probe=%v", opts.StateDir, opts.TemplatesDir, opts.RenderOnly, opts.ValidateOnly, opts.ProbeURL != "")

	mode, err := runtimecfg.ParseRuntimeMode(opts.Mode)
	if err != nil {
		return userError{Message: "Invalid runtime mode. Expected: real or simulation", Err: err}
	}
	rcfg := runtimecfg.RuntimeConfig{Mode: mode}
	log.Printf("mode=%s", rcfg.Mode)
	rt := buildRuntime(rcfg)
	_ = rt

	rts := newRuntimeStore(string(rcfg.Mode))
	rts.addEvent("AGENT_START", "Inside agent started", map[string]interface{}{"mode": string(rcfg.Mode)})
	apiCtx, apiCancel := context.WithCancel(context.Background())
	defer apiCancel()
	if strings.TrimSpace(opts.RuntimeAPIAddr) != "" {
		if _, err := startRuntimeAPIServer(apiCtx, opts.RuntimeAPIAddr, rts); err != nil {
			return userError{Message: "Runtime API failed to start (must bind to localhost only)", Err: err}
		}
		rts.addEvent("API_LISTEN", "Runtime API listening on "+strings.TrimSpace(opts.RuntimeAPIAddr), map[string]interface{}{"addr": strings.TrimSpace(opts.RuntimeAPIAddr)})
		log.Printf("runtime_api_listen=%s", strings.TrimSpace(opts.RuntimeAPIAddr))
	}

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
			return userError{Message: "Failed to write report file", Err: err}
		}
		fmt.Printf("[ShadowNet] Report written: %s\n", opts.ReportOut)
		return nil
	}

	if opts.ImportPath != "" {
		fmt.Printf("[ShadowNet] Importing bundle: %s\n", filepath.Base(opts.ImportPath))
		ageIdentity, err := loadAgeIdentity(opts.AgeIdentity, filepath.Join(keysDir, "age_identity.txt"))
		if err != nil {
			return userError{Message: "Missing or invalid age identity (required to decrypt bundles)", Err: err}
		}
		trustedKeys, err := parseTrustedSignerKeys(opts.TrustedSignerPubsB64, filepath.Join(keysDir, "trusted_signers.txt"))
		if err != nil {
			return userError{Message: "Missing or invalid trusted signer keys (required to verify bundles)", Err: err}
		}
		importer := importctl.NewImporterWithTemplates(store, templateStore, trustedKeys, ageIdentity)
		payload, err := importer.ImportFile(opts.ImportPath)
		if err != nil {
			return userError{Message: "Bundle import failed (signature/decryption/format)", Err: err}
		}
		if err := importer.ProcessAndStore(payload); err != nil {
			return userError{Message: "Bundle store failed", Err: err}
		}
		fmt.Printf("[Bundle] Import OK: profiles=%d templates=%d\n", len(payload.Profiles), len(payload.Templates))
		log.Printf("import ok profiles=%d revocations=%d templates=%d", len(payload.Profiles), len(payload.Revocations), len(payload.Templates))
	}

	allProfiles, err := store.Load()
	if err != nil {
		if strings.Contains(err.Error(), "decryption failed (wrong key or corrupted data)") {
			return userError{Message: "Failed to decrypt local store (wrong key or corrupted file). If you lost the key, delete the state directory to reset.", Err: err}
		}
		return userError{Message: "Failed to load local profiles store", Err: err}
	}
	log.Printf("profiles_loaded=%d", len(allProfiles))

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
		return userError{
			Message: "No valid profiles found.\n→ Import a trusted bundle first (or enable profiles in your store).",
			Err:     nil,
		}
	}

	engine := policy.Engine{MaxBurstFailures: 3}
	ranked := engine.RankProfiles(candidates)
	if len(ranked) == 0 {
		return userError{Message: "No viable profiles (all are in cooldown due to repeated failures). Try again later or import a fresh bundle.", Err: nil}
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
	policyCandidates := func() []string {
		out := make([]string, 0, len(ranked))
		for i := 0; i < len(ranked) && i < 5; i++ {
			out = append(out, ranked[i].ID)
		}
		return out
	}()
	rts.addEvent("POLICY_DECISION", "Policy ranked profiles", map[string]interface{}{
		"candidates":  policyCandidates,
		"selected":    selected.ID,
		"confidence":  policyConfidence,
		"reason":      policyReason,
		"fallbacks":   fallbackCandidates,
		"max_burst":   engine.MaxBurstFailures,
		"profile_cnt": len(candidates),
	})

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
		log.Printf("[policy] candidates=%v policy_confidence=%.2f", policyCandidates, policyConfidence)
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
		rts.addEvent("ORCHESTRATOR_DECISION", "Orchestrator selected profile", map[string]interface{}{
			"action":     "select_profile",
			"candidates": policyCandidates,
			"selected":   res.SelectedProfile.ID,
			"confidence": res.Confidence,
			"source":     res.Source,
			"reason":     res.Reason,
		})
		log.Printf("[orchestrator] decision=%s profile=%s confidence=%.2f reason=%q", res.Source, res.SelectedProfile.ID, res.Confidence, res.Reason)
	}

	log.Printf("select ok: profile=%s source=%s confidence=%.2f reason=%s fallbacks=%v", finalSelected.ID, finalSource, finalConfidence, finalReason, fallbackCandidates)
	fmt.Printf("[Profile] Selected: %s\n", finalSelected.ID)
	rts.setStatus("connecting")
	rts.setActiveProfile(finalSelected.ID)
	rts.addEvent("PROFILE_SWITCH", "Selected profile "+finalSelected.ID, map[string]interface{}{
		"selected":   finalSelected.ID,
		"source":     finalSource,
		"confidence": finalConfidence,
		"reason":     finalReason,
		"fallbacks":  fallbackCandidates,
	})

	ctrl := sbctl.NewController(runtimeDir, opts.SingBoxBin)

	probeEnabled := strings.TrimSpace(opts.ProbeURL) != ""
	probeProxyURL := opts.ProbeProxyAddr
	if !strings.Contains(probeProxyURL, "://") {
		probeProxyURL = "http://" + probeProxyURL
	}

	renderOpts := sbctl.RenderOptions{}
	if probeEnabled {
		renderOpts.ProbeListenAddr = opts.ProbeProxyAddr
		renderOpts.DisableTun = true
	}

	ordered := make([]profile.Profile, 0, len(ranked))
	ordered = append(ordered, finalSelected)
	for _, p := range ranked {
		if p.ID == finalSelected.ID {
			continue
		}
		ordered = append(ordered, p)
	}

	maxAttempts := opts.MaxAttempts
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	if maxAttempts > len(ordered) {
		maxAttempts = len(ordered)
	}

	var attempts []attemptState
	var lastState agentState
	knownProfiles := make(map[string]struct{}, len(ordered))
	for _, p := range ordered {
		knownProfiles[p.ID] = struct{}{}
	}
	switches := 0
	loopGuard := map[string]int{}

	if err := validateAction("select_profile", finalSelected.ID, knownProfiles, switches, 6, loopGuard, maxAttempts); err != nil {
		return fmt.Errorf("action rejected: %w", err)
	}

	prevProfileID := finalSelected.ID
	for i := 0; i < maxAttempts; i++ {
		p := ordered[i]
		switches++
		if err := validateAction("switch_profile", p.ID, knownProfiles, switches, 6, loopGuard, maxAttempts); err != nil {
			return fmt.Errorf("action rejected: %w", err)
		}
		rts.setActiveProfile(p.ID)
		if i > 0 {
			fmt.Printf("[Profile] Fallback: %s\n", p.ID)
			rts.addEvent("PROFILE_SWITCH", "Trying fallback profile "+p.ID, map[string]interface{}{
				"from":    prevProfileID,
				"to":      p.ID,
				"reason":  "fallback",
				"attempt": i + 1,
			})
		}
		prevProfileID = p.ID
		fmt.Printf("[ShadowNet] Starting agent... (attempt %d/%d)\n", i+1, maxAttempts)

		templateText, err := resolveTemplateText(p, templateStore, opts.TemplatesDir)
		if err != nil {
			return userError{Message: "Template resolution failed for selected profile", Err: err}
		}

		rendered, err := sbctl.RenderConfigToFileWithOptions(p, templateText, configPath, renderOpts)
		if err != nil {
			return userError{Message: "Failed to render sing-box config for selected profile", Err: err}
		}
		log.Printf("render ok: config=%s profile=%s family=%s", rendered.ConfigPath, p.ID, p.Family)
		rts.addEvent("CONFIG_RENDER", "Rendered config for profile "+p.ID, map[string]interface{}{
			"profile": p.ID,
			"family":  p.Family,
			"config":  rendered.ConfigPath,
		})

		if opts.RenderOnly {
			fmt.Printf("[Config] Rendered: %s\n", rendered.ConfigPath)
			st := agentState{
				StateDir:           opts.StateDir,
				ProfilesLoaded:     len(allProfiles),
				SelectedProfileID:  p.ID,
				SelectionReason:    finalReason,
				FallbackCandidates: fallbackCandidates,
				ConfigPath:         rendered.ConfigPath,
				SingBoxBinary:      ctrl.BinaryPath,
				SingBoxPID:         0,
				Status:             "rendered",
				Probe:              e2e.ProbeResult{Status: "skipped", Reason: e2e.ReasonUnknown, TargetURL: opts.ProbeURL, ProxyURL: probeProxyURL, ObservedAt: time.Now().Unix()},
				UpdatedAtUnix:      time.Now().Unix(),
			}
			return writeState(statePath, st)
		}

		if opts.ValidateOnly {
			fmt.Printf("[Config] Validating: %s\n", rendered.ConfigPath)
			st := agentState{
				StateDir:           opts.StateDir,
				ProfilesLoaded:     len(allProfiles),
				SelectedProfileID:  p.ID,
				SelectionReason:    finalReason,
				FallbackCandidates: fallbackCandidates,
				ConfigPath:         rendered.ConfigPath,
				SingBoxBinary:      ctrl.BinaryPath,
				SingBoxPID:         0,
				Status:             "validated",
				Probe:              e2e.ProbeResult{Status: "skipped", Reason: e2e.ReasonUnknown, TargetURL: opts.ProbeURL, ProxyURL: probeProxyURL, ObservedAt: time.Now().Unix()},
				UpdatedAtUnix:      time.Now().Unix(),
			}
			if err := ctrl.ValidateConfig(rendered.ConfigPath); err != nil {
				if errors.Is(err, sbctl.ErrBinaryNotFound) {
					_ = writeState(statePath, st)
					return userError{Message: "sing-box binary not found.\n→ Install sing-box or set --sing-box-bin to its path.", Err: err}
				}
				return userError{Message: "sing-box rejected the rendered config", Err: err}
			}
			log.Printf("[config] accepted config=%s", rendered.ConfigPath)
			rts.addEvent("CONFIG_ACCEPTED", "sing-box check ok", map[string]interface{}{"profile": p.ID, "config": rendered.ConfigPath})
			return writeState(statePath, st)
		}

		log.Printf("[sing-box] starting attempt=%d profile=%s", i+1, p.ID)
		fmt.Printf("[Connection] Starting...\n")
		rts.addEvent("SINGBOX_START", "Starting sing-box", map[string]interface{}{"profile": p.ID, "attempt": i + 1})
		if err := ctrl.ApplyAndReload(string(rendered.ConfigBytes)); err != nil {
			if errors.Is(err, sbctl.ErrBinaryNotFound) {
				st := agentState{
					StateDir:           opts.StateDir,
					ProfilesLoaded:     len(allProfiles),
					SelectedProfileID:  p.ID,
					SelectionReason:    finalReason,
					FallbackCandidates: fallbackCandidates,
					ConfigPath:         rendered.ConfigPath,
					SingBoxBinary:      ctrl.BinaryPath,
					SingBoxPID:         0,
					Status:             "failed",
					Probe:              e2e.ProbeResult{Status: "failed", Reason: e2e.ReasonBinaryMissing, TargetURL: opts.ProbeURL, ProxyURL: probeProxyURL, Error: err.Error(), ObservedAt: time.Now().Unix()},
					UpdatedAtUnix:      time.Now().Unix(),
				}
				_ = writeState(statePath, st)
				rts.addEvent("SINGBOX_START_FAILED", "sing-box binary missing", map[string]interface{}{"profile": p.ID, "reason": "BINARY_MISSING"})
				rts.addEvent("CONNECTION_FAIL", "sing-box binary missing", map[string]interface{}{"profile": p.ID, "reason": "BINARY_MISSING"})
				return userError{Message: "sing-box binary not found.\n→ Install sing-box or set --sing-box-bin to its path.", Err: err}
			}
			tail := readTailBytes(ctrl.StderrPath(), 64*1024)
			reason := e2e.ClassifyError(err, tail)
			probe := e2e.ProbeResult{
				Status:     "failed",
				Reason:     reason,
				TargetURL:  opts.ProbeURL,
				ProxyURL:   probeProxyURL,
				Error:      err.Error(),
				DurationMS: 0,
				ObservedAt: time.Now().Unix(),
			}
			attempts = append(attempts, attemptState{
				Attempt:    i + 1,
				ProfileID:  p.ID,
				Family:     string(p.Family),
				ConfigPath: rendered.ConfigPath,
				SingBoxPID: 0,
				Probe:      probe,
			})
			log.Printf("[sing-box] start failed attempt=%d profile=%s reason=%s err=%s", i+1, p.ID, reason, sanitizeLogText(err.Error()))
			rts.addFailure(string(reason))
			rts.addEvent("SINGBOX_START_FAILED", "sing-box start failed: "+string(reason), map[string]interface{}{"profile": p.ID, "reason": string(reason), "attempt": i + 1})
			rts.addEvent("CONNECTION_FAIL", "sing-box start failed", map[string]interface{}{"profile": p.ID, "reason": string(reason), "attempt": i + 1})
			_ = ctrl.Stop()
			lastState = agentState{
				StateDir:           opts.StateDir,
				ProfilesLoaded:     len(allProfiles),
				SelectedProfileID:  p.ID,
				SelectionReason:    finalReason,
				FallbackCandidates: fallbackCandidates,
				ConfigPath:         rendered.ConfigPath,
				SingBoxBinary:      ctrl.BinaryPath,
				SingBoxPID:         0,
				Status:             "failed",
				Probe:              probe,
				Attempts:           attempts,
				UpdatedAtUnix:      time.Now().Unix(),
			}
			continue
		}

		pid := ctrl.PID()
		log.Printf("[config] accepted config=%s", rendered.ConfigPath)
		log.Printf("[sing-box] started pid=%d attempt=%d profile=%s", pid, i+1, p.ID)
		rts.addEvent("SINGBOX_STARTED", fmt.Sprintf("sing-box started pid=%d", pid), map[string]interface{}{"profile": p.ID, "pid": pid, "attempt": i + 1})

		probe := e2e.ProbeResult{Status: "skipped", Reason: e2e.ReasonUnknown, TargetURL: opts.ProbeURL, ProxyURL: probeProxyURL, ObservedAt: time.Now().Unix()}
		if probeEnabled {
			fmt.Printf("[Connection] Testing...\n")
			waitCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			waitErr := waitForTCPListen(waitCtx, opts.ProbeProxyAddr)
			cancel()
			if waitErr != nil {
				probe = e2e.ProbeResult{
					Status:     "failed",
					Reason:     e2e.ReasonTimeout,
					TargetURL:  opts.ProbeURL,
					ProxyURL:   probeProxyURL,
					Error:      "proxy not ready: " + waitErr.Error(),
					DurationMS: 0,
					ObservedAt: time.Now().Unix(),
				}
				log.Printf("[probe] failed reason=%s err=%s", probe.Reason, sanitizeLogText(probe.Error))
				rts.addFailure(string(probe.Reason))
				rts.addEvent("PROBE_FAILED", "HTTP probe failed: "+string(probe.Reason), map[string]interface{}{"profile": p.ID, "reason": string(probe.Reason)})
				rts.addEvent("CONNECTION_FAIL", "HTTP probe failed", map[string]interface{}{"profile": p.ID, "reason": string(probe.Reason)})
			} else {
				log.Printf("[probe] start target=%s via=%s", opts.ProbeURL, probeProxyURL)
				ctx, cancel := context.WithTimeout(context.Background(), time.Duration(opts.ProbeTimeoutMS)*time.Millisecond)
				probe = e2e.HTTPProxyProbe(ctx, probeProxyURL, opts.ProbeURL)
				cancel()
				if probe.Status != "ok" {
					tail := readTailBytes(ctrl.StderrPath(), 64*1024)
					probe.Reason = e2e.ClassifyError(errors.New(probe.Error), tail)
					log.Printf("[probe] failed reason=%s err=%s", probe.Reason, probe.Error)
					rts.addFailure(string(probe.Reason))
					rts.addEvent("PROBE_FAILED", "HTTP probe failed: "+string(probe.Reason), map[string]interface{}{"profile": p.ID, "reason": string(probe.Reason)})
					rts.addEvent("CONNECTION_FAIL", "HTTP probe failed", map[string]interface{}{"profile": p.ID, "reason": string(probe.Reason)})
				} else {
					log.Printf("[probe] ok status=%d duration_ms=%d", probe.HTTPStatus, probe.DurationMS)
					log.Printf("[network] outbound connected")
					fmt.Printf("[Connection] SUCCESS\n")
					rts.setLatencyMs(probe.DurationMS)
					rts.setStatus("connected")
					rts.addEvent("PROBE_OK", fmt.Sprintf("HTTP probe ok status=%d latency_ms=%d", probe.HTTPStatus, probe.DurationMS), map[string]interface{}{"profile": p.ID, "http_status": probe.HTTPStatus, "latency_ms": probe.DurationMS})
					rts.addEvent("CONNECTION_SUCCESS", "Connection validated by HTTP probe", map[string]interface{}{"profile": p.ID, "http_status": probe.HTTPStatus, "latency_ms": probe.DurationMS})
				}
			}
		}

		attempts = append(attempts, attemptState{
			Attempt:    i + 1,
			ProfileID:  p.ID,
			Family:     string(p.Family),
			ConfigPath: rendered.ConfigPath,
			SingBoxPID: pid,
			Probe:      probe,
		})

		if probeEnabled && probe.Status != "ok" {
			_ = ctrl.Stop()
			lastState = agentState{
				StateDir:           opts.StateDir,
				ProfilesLoaded:     len(allProfiles),
				SelectedProfileID:  p.ID,
				SelectionReason:    finalReason,
				FallbackCandidates: fallbackCandidates,
				ConfigPath:         rendered.ConfigPath,
				SingBoxBinary:      ctrl.BinaryPath,
				SingBoxPID:         pid,
				Status:             "failed",
				Probe:              probe,
				Attempts:           attempts,
				UpdatedAtUnix:      time.Now().Unix(),
			}
			continue
		}

		st := agentState{
			StateDir:           opts.StateDir,
			ProfilesLoaded:     len(allProfiles),
			SelectedProfileID:  p.ID,
			SelectionReason:    finalReason,
			FallbackCandidates: fallbackCandidates,
			ConfigPath:         rendered.ConfigPath,
			SingBoxBinary:      ctrl.BinaryPath,
			SingBoxPID:         pid,
			Status:             "running",
			Probe:              probe,
			Attempts:           attempts,
			UpdatedAtUnix:      time.Now().Unix(),
		}
		if err := writeState(statePath, st); err != nil {
			return userError{Message: "Failed to write state file", Err: err}
		}
		if !opts.RuntimeAPIKeepAlive || strings.TrimSpace(opts.RuntimeAPIAddr) == "" {
			return nil
		}
		rts.addEvent("AGENT_HOLD", "Keeping agent alive for runtime API", map[string]interface{}{"addr": strings.TrimSpace(opts.RuntimeAPIAddr)})
		fmt.Printf("[ShadowNet Dashboard] Listening on http://%s\n", strings.TrimSpace(opts.RuntimeAPIAddr))
		log.Printf("[api] keepalive enabled; press Ctrl+C to stop")
		sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		<-sigCtx.Done()
		stop()
		_ = ctrl.Stop()
		rts.setStatus("disconnected")
		apiCancel()
		return nil
	}

	if lastState.UpdatedAtUnix == 0 {
		lastState = agentState{
			StateDir:           opts.StateDir,
			ProfilesLoaded:     len(allProfiles),
			SelectedProfileID:  finalSelected.ID,
			SelectionReason:    finalReason,
			FallbackCandidates: fallbackCandidates,
			ConfigPath:         configPath,
			SingBoxBinary:      ctrl.BinaryPath,
			SingBoxPID:         0,
			Status:             "failed",
			Probe:              e2e.ProbeResult{Status: "failed", Reason: e2e.ReasonUnknown, TargetURL: opts.ProbeURL, ProxyURL: probeProxyURL, Error: "no attempts completed", ObservedAt: time.Now().Unix()},
			Attempts:           attempts,
			UpdatedAtUnix:      time.Now().Unix(),
		}
	}
	_ = writeState(statePath, lastState)
	return userError{
		Message: fmt.Sprintf("Connection failed after %d attempt(s).\n→ Last reason: %s\n→ Try importing a newer bundle or re-running to try a different profile.", len(attempts), lastState.Probe.Reason),
		Err:     nil,
	}
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

func readTailBytes(path string, maxBytes int64) string {
	if path == "" || maxBytes <= 0 {
		return ""
	}
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return ""
	}
	size := st.Size()
	var off int64
	if size > maxBytes {
		off = size - maxBytes
	}
	if _, err := f.Seek(off, 0); err != nil {
		return ""
	}
	b, err := io.ReadAll(f)
	if err != nil {
		return ""
	}
	return string(b)
}

func waitForTCPListen(ctx context.Context, addr string) error {
	d := net.Dialer{Timeout: 250 * time.Millisecond}
	t := time.NewTicker(200 * time.Millisecond)
	defer t.Stop()
	for {
		c, err := d.DialContext(ctx, "tcp", addr)
		if err == nil {
			_ = c.Close()
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
		}
	}
}

func validateAction(action string, profileID string, known map[string]struct{}, switchCount int, maxPerMinute int, loopGuard map[string]int, maxAttempts int) error {
	if strings.TrimSpace(action) == "" {
		return fmt.Errorf("missing action")
	}
	if _, ok := known[profileID]; !ok {
		return fmt.Errorf("unknown profile %q", profileID)
	}
	if maxPerMinute > 0 && switchCount > maxPerMinute {
		return fmt.Errorf("rate limit exceeded")
	}
	if action == "switch_profile" {
		loopGuard[profileID]++
		if maxAttempts > 0 && loopGuard[profileID] > maxAttempts {
			return fmt.Errorf("loop protection triggered")
		}
	}
	return nil
}

func sanitizeLogText(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	// Keep error logs actionable while avoiding leaking config paths/endpoints.
	s = strings.ReplaceAll(s, `\`, "/")
	s = strings.ReplaceAll(s, "http://", "")
	s = strings.ReplaceAll(s, "https://", "")
	if idx := strings.Index(s, "/"); idx >= 0 {
		s = s[:idx] + "/…"
	}
	if len(s) > 200 {
		s = s[:200] + "…"
	}
	return s
}

type Runtime struct {
	Detector detector.Detector
	Mesh     mesh.Mesh
	Signal   signalrx.SignalReceiver
}

func buildRuntime(cfg runtimecfg.RuntimeConfig) Runtime {
	var d detector.Detector
	var m mesh.Mesh
	var s signalrx.SignalReceiver

	if cfg.Mode == runtimecfg.ModeSim {
		d = detsim.New(orchestrator.NetworkState{})
		m = meshsim.New(16)
		s = signalsim.New(8)
		log.Printf("[detector] simulation active")
		log.Printf("[mesh] simulation active")
		log.Printf("[signalrx] simulation active")
	} else {
		d = detreal.New(detreal.Config{})
		m = meshreal.New()
		s = signalreal.New()
		log.Printf("[detector] real implementation active")
		log.Printf("[mesh] real implementation active")
		log.Printf("[signalrx] real implementation active")
	}

	if d.RuntimeMode() != cfg.Mode {
		panic("detector mode mismatch")
	}
	if m.RuntimeMode() != cfg.Mode {
		panic("mesh mode mismatch")
	}
	if s.RuntimeMode() != cfg.Mode {
		panic("signalrx mode mismatch")
	}

	return Runtime{
		Detector: d,
		Mesh:     m,
		Signal:   s,
	}
}
