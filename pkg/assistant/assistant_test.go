package assistant

import (
	"context"
	"testing"
	"time"

	"github.com/kaveh/sunlionet-agent/pkg/aipolicy"
)

type captureProvider struct {
	lastPrompt string
	out        string
}

func (p *captureProvider) Complete(ctx context.Context, prompt string) (string, error) {
	_ = ctx
	p.lastPrompt = prompt
	return p.out, nil
}

func TestControllerEnforcesPolicy(t *testing.T) {
	pol := &aipolicy.Policy{
		Grants: []aipolicy.Grant{
			aipolicy.NewGrant(aipolicy.Scope{Type: aipolicy.ScopeChat, ID: "c1"}, []aipolicy.Action{aipolicy.ActionSummarize}, 10*time.Minute),
		},
	}
	local := &captureProvider{out: "ok"}
	c := &Controller{Local: local, Audit: NoopAuditSink{}}

	_, err := c.Invoke(context.Background(), pol, InvokeRequest{
		Scope:       aipolicy.Scope{Type: aipolicy.ScopeChat, ID: "c1"},
		Action:      aipolicy.ActionTranslate,
		Backend:     BackendLocal,
		Redaction:   RedactionOff,
		UserGesture: true,
		Items:       []Item{{Role: RoleUser, Text: "hi"}},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestControllerRedactsRemote(t *testing.T) {
	pol := &aipolicy.Policy{
		Grants: []aipolicy.Grant{
			aipolicy.NewGrant(aipolicy.Scope{Type: aipolicy.ScopeChat, ID: "c1"}, []aipolicy.Action{aipolicy.ActionSummarize}, 10*time.Minute),
		},
	}
	remote := &captureProvider{out: "ok"}
	c := &Controller{Remote: remote, Audit: NoopAuditSink{}}

	_, err := c.Invoke(context.Background(), pol, InvokeRequest{
		Scope:       aipolicy.Scope{Type: aipolicy.ScopeChat, ID: "c1"},
		Action:      aipolicy.ActionSummarize,
		Backend:     BackendRemote,
		Redaction:   RedactionStrict,
		UserGesture: true,
		Items:       []Item{{Role: RoleUser, Text: "email me at a@b.com https://x.test 12345 AGE-SECRET-KEY-1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq A6EHv_POEL4dcN0Y50vAmWfk1jCbpQ1fHdyGZBJVMbg 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}},
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if remote.lastPrompt == "" {
		t.Fatalf("expected prompt")
	}
	if want := "[redacted_email]"; !contains(remote.lastPrompt, want) {
		t.Fatalf("expected %q in prompt", want)
	}
	if want := "[redacted_url]"; !contains(remote.lastPrompt, want) {
		t.Fatalf("expected %q in prompt", want)
	}
	if want := "[redacted_digits]"; !contains(remote.lastPrompt, want) {
		t.Fatalf("expected %q in prompt", want)
	}
	if want := "[redacted_age_secret]"; !contains(remote.lastPrompt, want) {
		t.Fatalf("expected %q in prompt", want)
	}
	if want := "[redacted_b64]"; !contains(remote.lastPrompt, want) {
		t.Fatalf("expected %q in prompt", want)
	}
	if want := "[redacted_hex]"; !contains(remote.lastPrompt, want) {
		t.Fatalf("expected %q in prompt", want)
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})())
}
