//go:build daemon

package main

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/kaveh/sunlionet-agent/pkg/detector"
	"github.com/kaveh/sunlionet-agent/pkg/identity"
)

func TestMakePersonaStarter_DoesNotCaptureLoopVar(t *testing.T) {
	seed32 := make([]byte, 32)
	seedB64 := base64.RawURLEncoding.EncodeToString(seed32)

	personas := []identity.Persona{
		{ID: "p1"},
		{ID: "p2"},
		{ID: "p3"},
	}

	got := make(chan string, len(personas))
	starts := make([]func(), 0, len(personas))

	for i := range personas {
		persona := personas[i]
		binding := identity.MailboxBinding{
			PersonaID: persona.ID,
			CreatedAt: 1,
			SeedB64:   seedB64,
			PhaseSec:  0,
		}
		start := makePersonaStarter(persona, persona.ID, binding, func(p *identity.Persona, pid identity.PersonaID, b identity.MailboxBinding) {
			got <- string(p.ID) + "|" + string(pid) + "|" + string(b.PersonaID)
		})
		starts = append(starts, start)
	}

	for i := range starts {
		starts[i]()
	}
	close(got)

	seen := map[string]struct{}{}
	for v := range got {
		seen[v] = struct{}{}
	}

	for _, p := range personas {
		want := string(p.ID) + "|" + string(p.ID) + "|" + string(p.ID)
		if _, ok := seen[want]; !ok {
			t.Fatalf("missing starter payload: %q (got=%v)", want, seen)
		}
	}
	if len(seen) != len(personas) {
		t.Fatalf("expected %d unique starters, got %d (%v)", len(personas), len(seen), seen)
	}
}

func TestSendAnomaly_DoesNotBlockWhenFull(t *testing.T) {
	ch := make(chan detector.Event, 1)
	ch <- detector.Event{Type: "a"}

	done := make(chan struct{})
	go func() {
		sendAnomaly(ch, detector.Event{Type: "b"})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(250 * time.Millisecond):
		t.Fatalf("sendAnomaly blocked on a full channel")
	}

	if got := len(ch); got != 1 {
		t.Fatalf("expected channel size to remain 1, got %d", got)
	}
}
