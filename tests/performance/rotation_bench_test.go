//go:build perf
// +build perf

package performance

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/kaveh/sunlionet-agent/pkg/detector"
	"github.com/kaveh/sunlionet-agent/pkg/policy"
	"github.com/kaveh/sunlionet-agent/pkg/profile"
	"github.com/kaveh/sunlionet-agent/pkg/sbctl"
)

type benchAdvisor struct{}

func (a *benchAdvisor) ProposeAction(string, profile.Profile, []profile.Profile, []detector.Event) (policy.Action, error) {
	return policy.Action{}, os.ErrInvalid
}

func repoRoot(b *testing.B) string {
	b.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		b.Fatalf("failed to get caller path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func BenchmarkRotationManager_Rotate_Fallback(b *testing.B) {
	root := repoRoot(b)

	tmp := b.TempDir()
	storePath := filepath.Join(tmp, "store.enc")
	store, err := profile.NewStore(storePath, []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		b.Fatalf("store: %v", err)
	}

	seeds := []profile.Profile{
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
	}
	if err := store.Save(seeds); err != nil {
		b.Fatalf("save: %v", err)
	}

	ctrl := sbctl.NewController(filepath.Join(tmp, "sb"), filepath.Join(tmp, "missing-sing-box"))
	gen := sbctl.NewConfigGenerator(filepath.Join(root, "templates"))
	rm := policy.NewRotationManager(&benchAdvisor{}, ctrl, gen, store)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rm.Rotate(context.Background(), nil)
	}
}
