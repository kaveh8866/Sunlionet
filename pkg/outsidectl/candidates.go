package outsidectl

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/kaveh/sunlionet-agent/pkg/profile"
)

type Candidate struct {
	Profile profile.Profile
	Score   float64
}

type IncludedEntry struct {
	ID     string         `json:"id"`
	Family profile.Family `json:"family"`
	Score  float64        `json:"score"`
}

type Exclusion struct {
	ID     string `json:"id"`
	Reason string `json:"reason"`
}

type SelectionManifest struct {
	CreatedAtUnix int64           `json:"created_at_unix"`
	Included      []IncludedEntry `json:"included"`
	Excluded      []Exclusion     `json:"excluded"`
}

type SelectionResult struct {
	CreatedAtUnix int64
	Included      []Candidate
	Excluded      []Exclusion
	Manifest      SelectionManifest
}

type SelectionParams struct {
	MaxProfiles  int
	MaxPerFamily int
	NowUnix      int64
}

func LoadCandidates(profilesPath string, profilesDir string) ([]profile.Profile, error) {
	var out []profile.Profile
	if strings.TrimSpace(profilesPath) != "" {
		ps, err := loadProfilesFile(profilesPath)
		if err != nil {
			return nil, err
		}
		out = append(out, ps...)
	}
	if strings.TrimSpace(profilesDir) != "" {
		ps, err := loadProfilesDir(profilesDir)
		if err != nil {
			return nil, err
		}
		out = append(out, ps...)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no candidates loaded: set --profiles or --profiles-dir")
	}
	return out, nil
}

func ValidateNormalizeAndScore(candidates []profile.Profile, now int64) ([]Candidate, []Exclusion) {
	seenIDs := map[string]bool{}
	var out []Candidate
	var excluded []Exclusion

	for _, p := range candidates {
		cp, err := profile.NormalizeForWire(p, now)
		if err != nil {
			excluded = append(excluded, Exclusion{ID: p.ID, Reason: err.Error()})
			continue
		}
		if seenIDs[cp.ID] {
			excluded = append(excluded, Exclusion{ID: cp.ID, Reason: "duplicate id"})
			continue
		}
		seenIDs[cp.ID] = true

		score := scoreCandidate(cp, now)
		out = append(out, Candidate{Profile: cp, Score: score})
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score == out[j].Score {
			return out[i].Profile.ID < out[j].Profile.ID
		}
		return out[i].Score > out[j].Score
	})
	return out, excluded
}

func SelectForBundle(candidates []Candidate, params SelectionParams) SelectionResult {
	now := params.NowUnix
	if params.MaxProfiles <= 0 {
		params.MaxProfiles = 20
	}
	if params.MaxPerFamily <= 0 {
		params.MaxPerFamily = params.MaxProfiles
	}

	byFamily := map[profile.Family][]Candidate{}
	for _, c := range candidates {
		byFamily[c.Profile.Family] = append(byFamily[c.Profile.Family], c)
	}
	for fam := range byFamily {
		sort.SliceStable(byFamily[fam], func(i, j int) bool {
			if byFamily[fam][i].Score == byFamily[fam][j].Score {
				return byFamily[fam][i].Profile.ID < byFamily[fam][j].Profile.ID
			}
			return byFamily[fam][i].Score > byFamily[fam][j].Score
		})
	}

	var families []profile.Family
	for fam := range byFamily {
		families = append(families, fam)
	}
	sort.SliceStable(families, func(i, j int) bool { return families[i] < families[j] })

	selected := make([]Candidate, 0, params.MaxProfiles)
	excluded := []Exclusion{}
	seenEndpoints := map[string]bool{}
	selectedPerFamily := map[profile.Family]int{}

	selectOne := func(c Candidate) bool {
		if selectedPerFamily[c.Profile.Family] >= params.MaxPerFamily {
			excluded = append(excluded, Exclusion{ID: c.Profile.ID, Reason: "cap per family"})
			return false
		}
		ek := endpointKey(c.Profile)
		if seenEndpoints[ek] {
			excluded = append(excluded, Exclusion{ID: c.Profile.ID, Reason: "duplicate endpoint"})
			return false
		}
		seenEndpoints[ek] = true
		selectedPerFamily[c.Profile.Family]++
		selected = append(selected, c)
		return true
	}

	if params.MaxProfiles > 1 && len(families) > 1 {
		for _, fam := range families {
			if len(selected) >= params.MaxProfiles {
				break
			}
			if len(byFamily[fam]) == 0 {
				continue
			}
			if selectOne(byFamily[fam][0]) {
				byFamily[fam] = byFamily[fam][1:]
			}
		}
	}

	rest := make([]Candidate, 0, len(candidates))
	for _, fam := range families {
		rest = append(rest, byFamily[fam]...)
	}
	sort.SliceStable(rest, func(i, j int) bool {
		if rest[i].Score == rest[j].Score {
			return rest[i].Profile.ID < rest[j].Profile.ID
		}
		return rest[i].Score > rest[j].Score
	})

	for _, c := range rest {
		if len(selected) >= params.MaxProfiles {
			excluded = append(excluded, Exclusion{ID: c.Profile.ID, Reason: "max profiles reached"})
			continue
		}
		_ = selectOne(c)
	}

	sort.SliceStable(selected, func(i, j int) bool { return selected[i].Profile.ID < selected[j].Profile.ID })
	manifest := SelectionManifest{
		CreatedAtUnix: now,
		Excluded:      excluded,
	}
	manifest.Included = make([]IncludedEntry, 0, len(selected))
	for _, s := range selected {
		manifest.Included = append(manifest.Included, IncludedEntry{
			ID:     s.Profile.ID,
			Family: s.Profile.Family,
			Score:  s.Score,
		})
	}
	return SelectionResult{
		CreatedAtUnix: now,
		Included:      selected,
		Excluded:      excluded,
		Manifest:      manifest,
	}
}

func loadProfilesFile(path string) ([]profile.Profile, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return nil, fmt.Errorf("empty profiles file")
	}
	dec := json.NewDecoder(strings.NewReader(trimmed))
	dec.DisallowUnknownFields()
	if strings.HasPrefix(trimmed, "[") {
		var ps []profile.Profile
		if err := dec.Decode(&ps); err != nil {
			return nil, err
		}
		if err := ensureEOF(dec); err != nil {
			return nil, err
		}
		return ps, nil
	}
	var p profile.Profile
	if err := dec.Decode(&p); err != nil {
		return nil, err
	}
	if err := ensureEOF(dec); err != nil {
		return nil, err
	}
	return []profile.Profile{p}, nil
}

func loadProfilesDir(dir string) ([]profile.Profile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(strings.ToLower(name), ".json") {
			files = append(files, filepath.Join(dir, name))
		}
	}
	sort.Strings(files)
	if len(files) == 0 {
		return nil, fmt.Errorf("no .json files found in %s", dir)
	}
	var out []profile.Profile
	for _, p := range files {
		raw, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		dec := json.NewDecoder(strings.NewReader(string(raw)))
		dec.DisallowUnknownFields()
		var prof profile.Profile
		if err := dec.Decode(&prof); err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		if err := ensureEOF(dec); err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		out = append(out, prof)
	}
	return out, nil
}

func ensureEOF(dec *json.Decoder) error {
	tok, err := dec.Token()
	if err == io.EOF {
		return nil
	}
	if err != nil {
		return err
	}
	return errors.New("trailing JSON content: " + fmt.Sprint(tok))
}

func endpointKey(p profile.Profile) string {
	return fmt.Sprintf("%s|%s|%d", p.Family, p.Endpoint.Host, p.Endpoint.Port)
}

func scoreCandidate(p profile.Profile, now int64) float64 {
	score := float64(p.Source.TrustLevel)
	score += float64(p.Priority) * 10.0
	ageSec := now - p.CreatedAt
	if ageSec < 0 {
		ageSec = 0
	}
	if ageSec < int64((7 * 24 * time.Hour).Seconds()) {
		score += 15
	} else if ageSec < int64((30 * 24 * time.Hour).Seconds()) {
		score += 5
	}
	if p.ExpiresAt != 0 && p.ExpiresAt-now < int64((48*time.Hour).Seconds()) {
		score -= 20
	}
	return score
}
