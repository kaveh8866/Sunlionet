package aipolicy

import (
	"testing"
	"time"
)

func TestPolicyIsAllowed(t *testing.T) {
	p := &Policy{
		Grants: []Grant{
			NewGrant(Scope{Type: ScopeChat, ID: "c1"}, []Action{ActionSummarize, ActionTranslate}, 5*time.Minute),
		},
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if !p.IsAllowed(time.Now(), Scope{Type: ScopeChat, ID: "c1"}, ActionSummarize) {
		t.Fatalf("expected allowed")
	}
	if p.IsAllowed(time.Now(), Scope{Type: ScopeChat, ID: "c1"}, ActionModerate) {
		t.Fatalf("expected denied")
	}
}
