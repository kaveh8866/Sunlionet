package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/kaveh/shadownet-agent/pkg/profile"
)

type Report struct {
	Schema          string   `json:"schema"`
	AppVersion      string   `json:"app_version"`
	GeneratedAtUnix int64    `json:"generated_at_unix"`
	GeneratedHour   int64    `json:"generated_hour_unix"`
	GoVersion       string   `json:"go_version"`
	GOOS            string   `json:"goos"`
	GOARCH          string   `json:"goarch"`
	Mode            string   `json:"mode"`
	Errors          []string `json:"errors,omitempty"`

	Summary Summary `json:"summary"`
}

type Summary struct {
	StatePresent        bool              `json:"state_present"`
	ProfilesLoaded      int               `json:"profiles_loaded"`
	SelectedFamily      string            `json:"selected_family,omitempty"`
	SingBoxRunning      bool              `json:"sing_box_running"`
	SelectionSource     string            `json:"selection_source,omitempty"`
	SelectionConfidence string            `json:"selection_confidence,omitempty"`
	ProfileFamilies     map[string]int    `json:"profile_families,omitempty"`
	ProfileHealth       ProfileHealthRoll `json:"profile_health,omitempty"`
}

type ProfileHealthRoll struct {
	CooldownCount int `json:"cooldown_count"`
	FailingCount  int `json:"failing_count"`
}

type agentState struct {
	ProfilesLoaded     int      `json:"profiles_loaded"`
	SelectedProfileID  string   `json:"selected_profile_id"`
	SelectionReason    string   `json:"selection_reason"`
	FallbackCandidates []string `json:"fallback_candidates"`
	SingBoxPID         int      `json:"sing_box_pid"`
	UpdatedAtUnix      int64    `json:"updated_at_unix"`
}

func Generate(stateDir string, masterKey []byte) Report {
	now := time.Now()
	out := Report{
		Schema:          "shadownet.report.v1",
		AppVersion:      Version,
		GeneratedAtUnix: now.Unix(),
		GeneratedHour:   now.Truncate(time.Hour).Unix(),
		GoVersion:       runtime.Version(),
		GOOS:            runtime.GOOS,
		GOARCH:          runtime.GOARCH,
		Mode:            "inside",
		Summary: Summary{
			ProfileFamilies: map[string]int{},
		},
	}

	if strings.TrimSpace(stateDir) == "" {
		out.Errors = append(out.Errors, "missing state_dir")
		return out
	}
	if len(masterKey) != 32 {
		out.Errors = append(out.Errors, "missing or invalid master_key")
		return out
	}

	st, ok, err := readAgentState(filepath.Join(stateDir, "state.json"))
	if err != nil {
		out.Errors = append(out.Errors, fmt.Sprintf("state_read_error: %v", err))
	}
	out.Summary.StatePresent = ok
	if ok {
		out.Summary.ProfilesLoaded = st.ProfilesLoaded
		out.Summary.SingBoxRunning = st.SingBoxPID != 0
		out.Summary.SelectionSource = parseSelectionSource(st.SelectionReason)
		out.Summary.SelectionConfidence = parseSelectionConfidence(st.SelectionReason)
	}

	storePath := filepath.Join(stateDir, "profiles.enc")
	store, err := profile.NewStore(storePath, masterKey)
	if err != nil {
		out.Errors = append(out.Errors, fmt.Sprintf("profiles_store_error: %v", err))
		return out
	}
	profiles, err := store.Load()
	if err != nil {
		out.Errors = append(out.Errors, fmt.Sprintf("profiles_load_error: %v", err))
		return out
	}

	for _, p := range profiles {
		out.Summary.ProfileFamilies[string(p.Family)]++
		if p.Health.CooldownUntil > time.Now().Unix() {
			out.Summary.ProfileHealth.CooldownCount++
		}
		if p.Health.ConsecutiveFails > 0 {
			out.Summary.ProfileHealth.FailingCount++
		}
	}

	if ok && st.SelectedProfileID != "" {
		for _, p := range profiles {
			if p.ID == st.SelectedProfileID {
				out.Summary.SelectedFamily = string(p.Family)
				break
			}
		}
	}

	return out
}

func WriteFile(path string, r Report) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("missing report output path")
	}
	raw, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func readAgentState(path string) (agentState, bool, error) {
	var st agentState
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return agentState{}, false, nil
		}
		return agentState{}, false, err
	}
	if err := json.Unmarshal(raw, &st); err != nil {
		return agentState{}, true, err
	}
	return st, true, nil
}

func parseSelectionSource(reason string) string {
	r := strings.ToLower(reason)
	if strings.Contains(r, "source=orchestrator") {
		return "orchestrator"
	}
	if strings.Contains(r, "source=policy") {
		return "policy"
	}
	return ""
}

func parseSelectionConfidence(reason string) string {
	r := strings.ToLower(reason)
	idx := strings.Index(r, "confidence=")
	if idx == -1 {
		return ""
	}
	rest := r[idx+len("confidence="):]
	var b strings.Builder
	for _, ch := range rest {
		if (ch >= '0' && ch <= '9') || ch == '.' {
			b.WriteRune(ch)
			continue
		}
		break
	}
	s := b.String()
	if s == "" {
		return ""
	}
	return s
}
