package integration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"filippo.io/age"
	"github.com/kaveh/sunlionet-agent/pkg/detector"
	"github.com/kaveh/sunlionet-agent/pkg/identity"
	"github.com/kaveh/sunlionet-agent/pkg/llm"
	"github.com/kaveh/sunlionet-agent/pkg/policy"
	"github.com/kaveh/sunlionet-agent/pkg/profile"
	"github.com/kaveh/sunlionet-agent/pkg/relay"
	"github.com/kaveh/sunlionet-agent/pkg/sbctl"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("failed to get caller path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

type errorAdvisor struct{ err error }

func (a *errorAdvisor) ProposeAction(string, profile.Profile, []profile.Profile, []detector.Event) (policy.Action, error) {
	return policy.Action{}, a.err
}

func TestRotationManager_EndToEndFallbackCycle_WritesConfig(t *testing.T) {
	root := repoRoot(t)

	tmp := t.TempDir()
	storePath := filepath.Join(tmp, "store.enc")
	store, err := profile.NewStore(storePath, []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("store: %v", err)
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
		t.Fatalf("save: %v", err)
	}

	ctrl := sbctl.NewController(filepath.Join(tmp, "sb"), filepath.Join(tmp, "missing-sing-box"))
	gen := sbctl.NewConfigGenerator(filepath.Join(root, "templates"))
	rm := policy.NewRotationManager(&errorAdvisor{err: os.ErrNotExist}, ctrl, gen, store)

	rm.Rotate(context.Background(), []detector.Event{
		{Type: detector.EventSNIBlockSuspected, Severity: detector.SeverityHigh, Timestamp: time.Now()},
	})

	configPath := filepath.Join(ctrl.ConfigDir, "config.json")
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("expected config.json to exist, stat error: %v", err)
	}
}

func TestRotationManager_LLMClientInvalidJSON_FallbackStillAppliesConfig(t *testing.T) {
	root := repoRoot(t)

	tmp := t.TempDir()
	storePath := filepath.Join(tmp, "store.enc")
	store, err := profile.NewStore(storePath, []byte("0123456789abcdef0123456789abcdef"))
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

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"content":"not-json"}`))
	}))
	defer mockServer.Close()

	llmClient := llm.NewLocalLlamaCPPClient(mockServer.URL, false)

	ctrl := sbctl.NewController(filepath.Join(tmp, "sb"), filepath.Join(tmp, "missing-sing-box"))
	gen := sbctl.NewConfigGenerator(filepath.Join(root, "templates"))
	rm := policy.NewRotationManager(llmClient, ctrl, gen, store)

	rm.Rotate(context.Background(), []detector.Event{
		{Type: detector.EventActiveResetSuspected, Severity: detector.SeverityHigh, Timestamp: time.Now()},
	})

	configPath := filepath.Join(ctrl.ConfigDir, "config.json")
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("expected config.json to exist, stat error: %v", err)
	}
}

func TestPhase4_DeviceLink_AndRelayRoundTrip(t *testing.T) {
	tmp := t.TempDir()

	r, err := relay.NewFileRelay(filepath.Join(tmp, "relay"), relay.FileRelayOptions{
		MaxPendingPerMailbox: 1000,
		MaxTotalPending:      5000,
	})
	if err != nil {
		t.Fatalf("NewFileRelay: %v", err)
	}
	srv, err := relay.NewServer("127.0.0.1:0", r, relay.ServerOptions{
		AllowNonLocal:          false,
		MinPoWBits:             12,
		IPRateLimitPerMin:      6000,
		MailboxRateLimitPerMin: 6000,
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	if err := srv.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	})

	p, err := identity.NewPersona()
	if err != nil {
		t.Fatalf("NewPersona: %v", err)
	}
	d, err := identity.NewDevice(p.ID, "phone")
	if err != nil {
		t.Fatalf("NewDevice: %v", err)
	}
	ageID, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("GenerateX25519Identity: %v", err)
	}
	ageRecipient := ageID.Recipient().String()
	req, err := identity.NewDeviceJoinRequestForDevice(p.ID, d.DeviceID, d.SignPubB64, ageRecipient)
	if err != nil {
		t.Fatalf("NewDeviceJoinRequestForDevice: %v", err)
	}
	pkg, err := identity.ApproveDeviceJoinRequest(p, req)
	if err != nil {
		t.Fatalf("ApproveDeviceJoinRequest: %v", err)
	}
	link, err := identity.NewDeviceLinkBundle(p, req, pkg)
	if err != nil {
		t.Fatalf("NewDeviceLinkBundle: %v", err)
	}
	blob, err := identity.EncryptDeviceLinkBundle(p, ageRecipient, link)
	if err != nil {
		t.Fatalf("EncryptDeviceLinkBundle: %v", err)
	}
	sas, err := identity.DeviceLinkSAS(blob)
	if err != nil {
		t.Fatalf("DeviceLinkSAS: %v", err)
	}
	if sas == "" {
		t.Fatalf("expected non-empty sas")
	}
	linkDec, err := identity.DecryptDeviceLink(blob, ageID.String())
	if err != nil {
		t.Fatalf("DecryptDeviceLink: %v", err)
	}
	st := identity.NewState()
	st.Devices = append(st.Devices, *d)
	if err := identity.UpsertDeviceFromJoinPackage(st, p.SignPubB64, &linkDec.JoinPackage); err != nil {
		t.Fatalf("UpsertDeviceFromJoinPackage: %v", err)
	}

	client := relay.NewHTTPClient("http://" + srv.Addr())
	mb := relay.MailboxID("mb_" + string(p.ID))
	id, err := client.Push(context.Background(), relay.PushRequest{
		Mailbox:  mb,
		Envelope: relay.Envelope("ciphertext"),
		PoWBits:  12,
	})
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	msgs, err := client.Pull(context.Background(), relay.PullRequest{Mailbox: mb, Limit: 10})
	if err != nil {
		t.Fatalf("Pull: %v", err)
	}
	if len(msgs) != 1 || msgs[0].ID != id {
		t.Fatalf("unexpected pull result: %+v", msgs)
	}
	if err := client.Ack(context.Background(), relay.AckRequest{Mailbox: mb, IDs: []relay.MessageID{id}}); err != nil {
		t.Fatalf("Ack: %v", err)
	}
	msgs, err = client.Pull(context.Background(), relay.PullRequest{Mailbox: mb, Limit: 10})
	if err != nil {
		t.Fatalf("Pull after ack: %v", err)
	}
	if len(msgs) != 0 {
		t.Fatalf("expected 0 messages after ack, got %d", len(msgs))
	}
}
