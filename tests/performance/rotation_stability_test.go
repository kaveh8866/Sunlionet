package performance

import (
	"context"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
	"time"

	"github.com/kaveh/shadownet-agent/pkg/detector"
	"github.com/kaveh/shadownet-agent/pkg/policy"
	"github.com/kaveh/shadownet-agent/pkg/profile"
	"github.com/kaveh/shadownet-agent/pkg/sbctl"
)

type perfAdvisor struct{}

func (a *perfAdvisor) ProposeAction(string, profile.Profile, []profile.Profile, []detector.Event) (policy.Action, error) {
	return policy.Action{}, context.DeadlineExceeded
}

func repoRootForTest(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("failed to get caller path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func newRotationManagerForPerf(t *testing.T) *policy.RotationManager {
	t.Helper()

	root := repoRootForTest(t)
	tmp := t.TempDir()

	storePath := filepath.Join(tmp, "store.enc")
	store, err := profile.NewStore(storePath, "0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	if err := store.Save([]profile.Profile{
		{
			ID:      "p1",
			Family:  profile.FamilyReality,
			Enabled: true,
			Endpoint: profile.Endpoint{
				Host: "127.0.0.1",
				Port: 443,
			},
			Capabilities: profile.Capabilities{Transport: "tcp"},
			Health:       profile.Health{SuccessEWMA: 0.9},
		},
	}); err != nil {
		t.Fatalf("save: %v", err)
	}

	ctrl := sbctl.NewController(filepath.Join(tmp, "sb"), filepath.Join(tmp, "missing-sing-box"))
	gen := sbctl.NewConfigGenerator(filepath.Join(root, "templates"))
	return policy.NewRotationManager(&perfAdvisor{}, ctrl, gen, store)
}

func TestDecisionLatencyP95_Under5s(t *testing.T) {
	rm := newRotationManagerForPerf(t)

	const n = 200
	durs := make([]time.Duration, 0, n)
	for i := 0; i < n; i++ {
		start := time.Now()
		rm.Rotate(context.Background(), nil)
		durs = append(durs, time.Since(start))
	}

	sort.Slice(durs, func(i, j int) bool { return durs[i] < durs[j] })
	idx := (n*95 + 99) / 100
	if idx <= 0 {
		idx = 1
	}
	p95 := durs[idx-1]
	if p95 > 5*time.Second {
		t.Fatalf("p95 too slow: %v", p95)
	}
}

func TestRotationStability_1000Switches_NoPanic(t *testing.T) {
	rm := newRotationManagerForPerf(t)
	for i := 0; i < 1000; i++ {
		rm.Rotate(context.Background(), nil)
	}
}
