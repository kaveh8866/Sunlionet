package ledgersync

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/rand"
	"testing"
	"time"

	"github.com/kaveh/sunlionet-agent/pkg/identity"
	"github.com/kaveh/sunlionet-agent/pkg/ledger"
	"github.com/kaveh/sunlionet-agent/pkg/mesh"
	"github.com/kaveh/sunlionet-agent/pkg/runtimecfg"
)

type inboxMesh struct {
	in chan []byte
}

func (m *inboxMesh) Broadcast(data []byte) error { return nil }

func (m *inboxMesh) Receive(ctx context.Context) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case b := <-m.in:
		return b, nil
	}
}

func (m *inboxMesh) RuntimeMode() runtimecfg.RuntimeMode { return runtimecfg.ModeSim }

func TestChaos_10Nodes_CriticalNotStarvedBySpam(t *testing.T) {
	const n = 10

	meshes := make([]*inboxMesh, 0, n)
	cryptos := make([]*mesh.Crypto, 0, n)
	ledgers := make([]*ledger.Ledger, 0, n)
	svcs := make([]*Service, 0, n)

	opts := Options{
		MaxHave: 0,
		MaxWant: 64,

		MaxEvents:             128,
		MaxEventBytes:         256 * 1024,
		MaxEventsMessageBytes: 512 * 1024,

		VerifyWorkers:   2,
		VerifyBatchSize: 64,

		SchedulerCriticalWeight:   5,
		SchedulerNormalWeight:     3,
		SchedulerBackgroundWeight: 1,

		SchedulerDrainPerReceive:   4,
		SchedulerDrainAfterRelay:   64,
		SchedulerMaxQueuedTotal:    256,
		SchedulerMaxQueuedPerPeer:  64,
		SchedulerMaxQueuedCritical: 16,
		SchedulerMaxQueuedNormal:   32,
		SchedulerMaxQueuedBg:       32,
	}

	for i := 0; i < n; i++ {
		m := &inboxMesh{in: make(chan []byte, 8192)}
		c, err := mesh.NewCrypto()
		if err != nil {
			t.Fatalf("NewCrypto %d: %v", i, err)
		}
		l := ledger.New()
		s, err := New(m, c, l, nil, nil, opts)
		if err != nil {
			t.Fatalf("New svc %d: %v", i, err)
		}
		meshes = append(meshes, m)
		cryptos = append(cryptos, c)
		ledgers = append(ledgers, l)
		svcs = append(svcs, s)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for i := 0; i < n; i++ {
		svc := svcs[i]
		go func() {
			for {
				err := svc.ReceiveOnce(ctx)
				if err != nil {
					if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
						return
					}
				}
			}
		}()
	}

	rng := rand.New(rand.NewSource(1))
	deliver := func(from, to int, pt []byte) {
		if from == to {
			return
		}
		if rng.Float64() < 0.10 {
			return
		}
		delay := time.Duration(rng.Intn(5)) * time.Millisecond
		dup := rng.Float64() < 0.05

		send := func(extra time.Duration) {
			msg, err := cryptos[from].EncryptPayload(pt, cryptos[to].PublicKey())
			if err != nil {
				return
			}
			raw, err := json.Marshal(msg)
			if err != nil {
				return
			}
			time.AfterFunc(delay+extra, func() {
				select {
				case meshes[to].in <- raw:
				default:
				}
			})
		}

		send(0)
		if dup {
			send(time.Duration(rng.Intn(5)) * time.Millisecond)
		}
	}

	type signer struct {
		author string
		pub    ed25519.PublicKey
		priv   ed25519.PrivateKey
	}

	makeSigner := func() (signer, error) {
		p, err := identity.NewPersona()
		if err != nil {
			return signer{}, err
		}
		pub, priv, err := p.SignKeypair()
		if err != nil {
			return signer{}, err
		}
		return signer{author: string(p.ID), pub: pub, priv: priv}, nil
	}

	makeSpamChain := func(s signer, count int) ([]ledger.Event, error) {
		out := make([]ledger.Event, 0, count)
		prev := ""
		for i := 1; i <= count; i++ {
			ev, err := ledger.NewSignedEvent(ledger.SignedEventInput{
				Author:     s.author,
				AuthorPub:  s.pub,
				AuthorPriv: s.priv,
				Seq:        uint64(i),
				Prev:       prev,
				Parents:    nil,
				Kind:       "gossip.bulk",
				Payload:    nil,
				CreatedAt:  time.Now(),
			})
			if err != nil {
				return nil, err
			}
			prev = ev.ID
			out = append(out, ev)
		}
		return out, nil
	}

	s1, err := makeSigner()
	if err != nil {
		t.Fatalf("signer1: %v", err)
	}
	s2, err := makeSigner()
	if err != nil {
		t.Fatalf("signer2: %v", err)
	}

	spam1, err := makeSpamChain(s1, 80)
	if err != nil {
		t.Fatalf("spam1: %v", err)
	}
	spam2, err := makeSpamChain(s2, 80)
	if err != nil {
		t.Fatalf("spam2: %v", err)
	}

	batchSize := 8
	sendBatches := func(from int, evs []ledger.Event) {
		for i := 0; i < len(evs); i += batchSize {
			j := i + batchSize
			if j > len(evs) {
				j = len(evs)
			}
			em := ledger.EventsMessage{SchemaVersion: ledger.SyncSchemaV1, Events: evs[i:j]}
			pt, err := encodeWire("events", "s", "ctx", em)
			if err != nil {
				t.Fatalf("encode spam wire: %v", err)
			}
			for to := 0; to < n; to++ {
				deliver(from, to, pt)
			}
		}
	}

	sendBatches(0, spam1)
	sendBatches(0, spam2)

	critSigner, err := makeSigner()
	if err != nil {
		t.Fatalf("crit signer: %v", err)
	}

	headsHash := base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	pl := ledger.SyncSummaryPayload{Context: "ctx", HeadsHash: headsHash, HeadCount: 1}
	rawPl, err := json.Marshal(pl)
	if err != nil {
		t.Fatalf("marshal critical payload: %v", err)
	}
	_, ref, err := ledger.InlinePayloadRef(rawPl)
	if err != nil {
		t.Fatalf("InlinePayloadRef: %v", err)
	}
	critical, err := ledger.NewSignedEvent(ledger.SignedEventInput{
		Author:     critSigner.author,
		AuthorPub:  critSigner.pub,
		AuthorPriv: critSigner.priv,
		Seq:        1,
		Kind:       ledger.KindSyncSummary,
		Payload:    rawPl,
		PayloadRef: ref,
		CreatedAt:  time.Now(),
	})
	if err != nil {
		t.Fatalf("new critical: %v", err)
	}
	if _, err := ledgers[0].Apply(critical, nil, nil); err != nil {
		t.Fatalf("apply critical on node0: %v", err)
	}

	cm := ledger.EventsMessage{SchemaVersion: ledger.SyncSchemaV1, Events: []ledger.Event{critical}}
	critPt, err := encodeWire("events", "s2", "ctx", cm)
	if err != nil {
		t.Fatalf("encode critical wire: %v", err)
	}
	for attempt := 0; attempt < 6; attempt++ {
		for to := 1; to < n; to++ {
			deliver(0, to, critPt)
		}
		time.Sleep(10 * time.Millisecond)
	}

	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for {
		allHave := true
		for i := 0; i < n; i++ {
			if !ledgers[i].Have(critical.ID) {
				allHave = false
				break
			}
		}
		if allHave {
			return
		}
		select {
		case <-deadline.C:
			miss := make([]int, 0, n)
			for i := 0; i < n; i++ {
				if !ledgers[i].Have(critical.ID) {
					miss = append(miss, i)
				}
			}
			t.Fatalf("critical event not applied on nodes: %v", miss)
		case <-tick.C:
		}
	}
}

func TestChaos_10Nodes_AttestationQuorumNotStarvedBySpam(t *testing.T) {
	const n = 10

	meshes := make([]*inboxMesh, 0, n)
	cryptos := make([]*mesh.Crypto, 0, n)
	ledgers := make([]*ledger.Ledger, 0, n)
	svcs := make([]*Service, 0, n)

	opts := Options{
		MaxHave: 0,
		MaxWant: 64,

		MaxEvents:             128,
		MaxEventBytes:         256 * 1024,
		MaxEventsMessageBytes: 512 * 1024,

		VerifyWorkers:   2,
		VerifyBatchSize: 64,

		SchedulerCriticalWeight:   5,
		SchedulerNormalWeight:     3,
		SchedulerBackgroundWeight: 1,

		SchedulerDrainPerReceive:   4,
		SchedulerDrainAfterRelay:   64,
		SchedulerMaxQueuedTotal:    256,
		SchedulerMaxQueuedPerPeer:  64,
		SchedulerMaxQueuedCritical: 16,
		SchedulerMaxQueuedNormal:   32,
		SchedulerMaxQueuedBg:       32,
	}

	ctxName := "ctx"
	w1, _ := identity.NewPersona()
	w1Pub, w1Priv, _ := w1.SignKeypair()
	w2, _ := identity.NewPersona()
	w2Pub, w2Priv, _ := w2.SignKeypair()
	w1Key := base64.RawURLEncoding.EncodeToString(w1Pub)
	w2Key := base64.RawURLEncoding.EncodeToString(w2Pub)

	pol := ledger.DefaultPolicy()
	pol.Trust = ledger.TrustPolicy{
		Witnesses:  map[string]map[string]int{ctxName: {w1Key: 1, w2Key: 1}},
		Thresholds: map[string]int{ctxName: 2},
	}

	for i := 0; i < n; i++ {
		m := &inboxMesh{in: make(chan []byte, 8192)}
		c, err := mesh.NewCrypto()
		if err != nil {
			t.Fatalf("NewCrypto %d: %v", i, err)
		}
		l := ledger.New()
		s, err := New(m, c, l, &pol, nil, opts)
		if err != nil {
			t.Fatalf("New svc %d: %v", i, err)
		}
		meshes = append(meshes, m)
		cryptos = append(cryptos, c)
		ledgers = append(ledgers, l)
		svcs = append(svcs, s)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()

	for i := 0; i < n; i++ {
		svc := svcs[i]
		go func() {
			for {
				err := svc.ReceiveOnce(ctx)
				if err != nil {
					if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
						return
					}
				}
			}
		}()
	}

	rng := rand.New(rand.NewSource(2))
	deliver := func(from, to int, pt []byte) {
		if from == to {
			return
		}
		if rng.Float64() < 0.10 {
			return
		}
		delay := time.Duration(rng.Intn(5)) * time.Millisecond
		dup := rng.Float64() < 0.05

		send := func(extra time.Duration) {
			msg, err := cryptos[from].EncryptPayload(pt, cryptos[to].PublicKey())
			if err != nil {
				return
			}
			raw, err := json.Marshal(msg)
			if err != nil {
				return
			}
			time.AfterFunc(delay+extra, func() {
				select {
				case meshes[to].in <- raw:
				default:
				}
			})
		}

		send(0)
		if dup {
			send(time.Duration(rng.Intn(5)) * time.Millisecond)
		}
	}

	type signer struct {
		author string
		pub    ed25519.PublicKey
		priv   ed25519.PrivateKey
	}

	makeSigner := func() (signer, error) {
		p, err := identity.NewPersona()
		if err != nil {
			return signer{}, err
		}
		pub, priv, err := p.SignKeypair()
		if err != nil {
			return signer{}, err
		}
		return signer{author: string(p.ID), pub: pub, priv: priv}, nil
	}

	makeSpamChain := func(s signer, count int) ([]ledger.Event, error) {
		out := make([]ledger.Event, 0, count)
		prev := ""
		for i := 1; i <= count; i++ {
			ev, err := ledger.NewSignedEvent(ledger.SignedEventInput{
				Author:     s.author,
				AuthorPub:  s.pub,
				AuthorPriv: s.priv,
				Seq:        uint64(i),
				Prev:       prev,
				Parents:    nil,
				Kind:       "gossip.bulk",
				Payload:    nil,
				CreatedAt:  time.Now(),
			})
			if err != nil {
				return nil, err
			}
			prev = ev.ID
			out = append(out, ev)
		}
		return out, nil
	}

	s1, err := makeSigner()
	if err != nil {
		t.Fatalf("signer1: %v", err)
	}
	s2, err := makeSigner()
	if err != nil {
		t.Fatalf("signer2: %v", err)
	}
	spam1, err := makeSpamChain(s1, 80)
	if err != nil {
		t.Fatalf("spam1: %v", err)
	}
	spam2, err := makeSpamChain(s2, 80)
	if err != nil {
		t.Fatalf("spam2: %v", err)
	}

	batchSize := 8
	sendBatches := func(from int, evs []ledger.Event) {
		for i := 0; i < len(evs); i += batchSize {
			j := i + batchSize
			if j > len(evs) {
				j = len(evs)
			}
			em := ledger.EventsMessage{SchemaVersion: ledger.SyncSchemaV1, Events: evs[i:j]}
			pt, err := encodeWire("events", "s", ctxName, em)
			if err != nil {
				t.Fatalf("encode spam wire: %v", err)
			}
			for to := 0; to < n; to++ {
				deliver(from, to, pt)
			}
		}
	}

	sendBatches(0, spam1)
	sendBatches(0, spam2)

	author, err := makeSigner()
	if err != nil {
		t.Fatalf("author signer: %v", err)
	}
	ev, err := ledger.NewSignedEvent(ledger.SignedEventInput{
		Author:     author.author,
		AuthorPub:  author.pub,
		AuthorPriv: author.priv,
		Seq:        1,
		Kind:       ledger.KindChatMessage,
		Payload:    json.RawMessage(`{"t":"x"}`),
		CreatedAt:  time.Now(),
	})
	if err != nil {
		t.Fatalf("new base event: %v", err)
	}
	if _, err := ledgers[0].Apply(ev, &pol, nil); err != nil {
		t.Fatalf("apply base event on node0: %v", err)
	}
	baseMsg := ledger.EventsMessage{SchemaVersion: ledger.SyncSchemaV1, Events: []ledger.Event{ev}}
	basePt, err := encodeWire("events", "base", ctxName, baseMsg)
	if err != nil {
		t.Fatalf("encode base wire: %v", err)
	}
	for attempt := 0; attempt < 6; attempt++ {
		for to := 1; to < n; to++ {
			deliver(0, to, basePt)
		}
		time.Sleep(10 * time.Millisecond)
	}
	waitBaseDeadline := time.NewTimer(3 * time.Second)
	defer waitBaseDeadline.Stop()
	waitBaseTick := time.NewTicker(10 * time.Millisecond)
	defer waitBaseTick.Stop()
	for {
		allHave := true
		for i := 0; i < n; i++ {
			if !ledgers[i].Have(ev.ID) {
				allHave = false
				break
			}
		}
		if allHave {
			break
		}
		select {
		case <-waitBaseDeadline.C:
			miss := make([]int, 0, n)
			for i := 0; i < n; i++ {
				if !ledgers[i].Have(ev.ID) {
					miss = append(miss, i)
				}
			}
			t.Fatalf("base event not applied on nodes: %v", miss)
		case <-waitBaseTick.C:
		}
	}

	a1pl := ledger.AttestationPayload{EventID: ev.ID, Context: ctxName}
	a1raw, _ := json.Marshal(a1pl)
	_, a1ref, _ := ledger.InlinePayloadRef(a1raw)
	a1, err := ledger.NewSignedEvent(ledger.SignedEventInput{
		Author:     string(w1.ID),
		AuthorPub:  w1Pub,
		AuthorPriv: w1Priv,
		Seq:        1,
		Kind:       ledger.KindWitnessAttest,
		Payload:    a1raw,
		PayloadRef: a1ref,
		CreatedAt:  time.Now(),
	})
	if err != nil {
		t.Fatalf("new attest 1: %v", err)
	}
	if _, err := ledgers[0].Apply(a1, &pol, nil); err != nil {
		t.Fatalf("apply attest 1 on node0: %v", err)
	}

	a2pl := ledger.AttestationPayload{EventID: ev.ID, Context: ctxName}
	a2raw, _ := json.Marshal(a2pl)
	_, a2ref, _ := ledger.InlinePayloadRef(a2raw)
	a2, err := ledger.NewSignedEvent(ledger.SignedEventInput{
		Author:     string(w2.ID),
		AuthorPub:  w2Pub,
		AuthorPriv: w2Priv,
		Seq:        1,
		Kind:       ledger.KindWitnessAttest,
		Payload:    a2raw,
		PayloadRef: a2ref,
		CreatedAt:  time.Now(),
	})
	if err != nil {
		t.Fatalf("new attest 2: %v", err)
	}
	if _, err := ledgers[0].Apply(a2, &pol, nil); err != nil {
		t.Fatalf("apply attest 2 on node0: %v", err)
	}

	am := ledger.EventsMessage{SchemaVersion: ledger.SyncSchemaV1, Events: []ledger.Event{a1, a2}}
	attPt, err := encodeWire("events", "att", ctxName, am)
	if err != nil {
		t.Fatalf("encode attest wire: %v", err)
	}
	for attempt := 0; attempt < 6; attempt++ {
		for to := 1; to < n; to++ {
			deliver(0, to, attPt)
		}
		time.Sleep(10 * time.Millisecond)
	}

	deadline := time.NewTimer(4 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for {
		allConfirmed := true
		for i := 0; i < n; i++ {
			if !ledgers[i].Have(a1.ID) || !ledgers[i].Have(a2.ID) || !ledgers[i].Have(ev.ID) {
				allConfirmed = false
				break
			}
			if _, ok := ledgers[i].Confirmed(ev.ID, ctxName, &pol); !ok {
				allConfirmed = false
				break
			}
		}
		if allConfirmed {
			return
		}
		select {
		case <-deadline.C:
			miss := make([]int, 0, n)
			for i := 0; i < n; i++ {
				if _, ok := ledgers[i].Confirmed(ev.ID, ctxName, &pol); !ok {
					miss = append(miss, i)
				}
			}
			t.Fatalf("attestation quorum not reached on nodes: %v", miss)
		case <-tick.C:
		}
	}
}

func TestChaos_10Nodes_CheckpointQuorumNotStarvedBySpam(t *testing.T) {
	const n = 10

	meshes := make([]*inboxMesh, 0, n)
	cryptos := make([]*mesh.Crypto, 0, n)
	ledgers := make([]*ledger.Ledger, 0, n)
	svcs := make([]*Service, 0, n)

	opts := Options{
		MaxHave: 0,
		MaxWant: 64,

		MaxEvents:             128,
		MaxEventBytes:         256 * 1024,
		MaxEventsMessageBytes: 512 * 1024,

		VerifyWorkers:   2,
		VerifyBatchSize: 64,

		SchedulerCriticalWeight:   5,
		SchedulerNormalWeight:     3,
		SchedulerBackgroundWeight: 1,

		SchedulerDrainPerReceive:   4,
		SchedulerDrainAfterRelay:   64,
		SchedulerMaxQueuedTotal:    256,
		SchedulerMaxQueuedPerPeer:  64,
		SchedulerMaxQueuedCritical: 16,
		SchedulerMaxQueuedNormal:   32,
		SchedulerMaxQueuedBg:       32,
	}

	ctxName := "ctx"
	w1, _ := identity.NewPersona()
	w1Pub, w1Priv, _ := w1.SignKeypair()
	w2, _ := identity.NewPersona()
	w2Pub, w2Priv, _ := w2.SignKeypair()
	w1Key := base64.RawURLEncoding.EncodeToString(w1Pub)
	w2Key := base64.RawURLEncoding.EncodeToString(w2Pub)

	pol := ledger.DefaultPolicy()
	pol.Trust = ledger.TrustPolicy{
		Witnesses:  map[string]map[string]int{ctxName: {w1Key: 1, w2Key: 1}},
		Thresholds: map[string]int{ctxName: 2},
	}

	for i := 0; i < n; i++ {
		m := &inboxMesh{in: make(chan []byte, 8192)}
		c, err := mesh.NewCrypto()
		if err != nil {
			t.Fatalf("NewCrypto %d: %v", i, err)
		}
		l := ledger.New()
		s, err := New(m, c, l, &pol, nil, opts)
		if err != nil {
			t.Fatalf("New svc %d: %v", i, err)
		}
		meshes = append(meshes, m)
		cryptos = append(cryptos, c)
		ledgers = append(ledgers, l)
		svcs = append(svcs, s)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()

	for i := 0; i < n; i++ {
		svc := svcs[i]
		go func() {
			for {
				err := svc.ReceiveOnce(ctx)
				if err != nil {
					if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
						return
					}
				}
			}
		}()
	}

	rng := rand.New(rand.NewSource(3))
	deliver := func(from, to int, pt []byte) {
		if from == to {
			return
		}
		if rng.Float64() < 0.10 {
			return
		}
		delay := time.Duration(rng.Intn(5)) * time.Millisecond
		dup := rng.Float64() < 0.05

		send := func(extra time.Duration) {
			msg, err := cryptos[from].EncryptPayload(pt, cryptos[to].PublicKey())
			if err != nil {
				return
			}
			raw, err := json.Marshal(msg)
			if err != nil {
				return
			}
			time.AfterFunc(delay+extra, func() {
				select {
				case meshes[to].in <- raw:
				default:
				}
			})
		}

		send(0)
		if dup {
			send(time.Duration(rng.Intn(5)) * time.Millisecond)
		}
	}

	type signer struct {
		author string
		pub    ed25519.PublicKey
		priv   ed25519.PrivateKey
	}

	makeSigner := func() (signer, error) {
		p, err := identity.NewPersona()
		if err != nil {
			return signer{}, err
		}
		pub, priv, err := p.SignKeypair()
		if err != nil {
			return signer{}, err
		}
		return signer{author: string(p.ID), pub: pub, priv: priv}, nil
	}

	makeSpamChain := func(s signer, count int) ([]ledger.Event, error) {
		out := make([]ledger.Event, 0, count)
		prev := ""
		for i := 1; i <= count; i++ {
			ev, err := ledger.NewSignedEvent(ledger.SignedEventInput{
				Author:     s.author,
				AuthorPub:  s.pub,
				AuthorPriv: s.priv,
				Seq:        uint64(i),
				Prev:       prev,
				Parents:    nil,
				Kind:       "gossip.bulk",
				Payload:    nil,
				CreatedAt:  time.Now(),
			})
			if err != nil {
				return nil, err
			}
			prev = ev.ID
			out = append(out, ev)
		}
		return out, nil
	}

	s1, err := makeSigner()
	if err != nil {
		t.Fatalf("signer1: %v", err)
	}
	s2, err := makeSigner()
	if err != nil {
		t.Fatalf("signer2: %v", err)
	}
	spam1, err := makeSpamChain(s1, 80)
	if err != nil {
		t.Fatalf("spam1: %v", err)
	}
	spam2, err := makeSpamChain(s2, 80)
	if err != nil {
		t.Fatalf("spam2: %v", err)
	}

	batchSize := 8
	sendBatches := func(from int, evs []ledger.Event) {
		for i := 0; i < len(evs); i += batchSize {
			j := i + batchSize
			if j > len(evs) {
				j = len(evs)
			}
			em := ledger.EventsMessage{SchemaVersion: ledger.SyncSchemaV1, Events: evs[i:j]}
			pt, err := encodeWire("events", "s", ctxName, em)
			if err != nil {
				t.Fatalf("encode spam wire: %v", err)
			}
			for to := 0; to < n; to++ {
				deliver(from, to, pt)
			}
		}
	}

	sendBatches(0, spam1)
	sendBatches(0, spam2)

	headsHash := base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	cpPl := ledger.CheckpointPayload{
		Context:    ctxName,
		HeadsHash:  headsHash,
		HeadsCount: 1,
	}
	cpRaw, _ := json.Marshal(cpPl)
	_, cpRef, _ := ledger.InlinePayloadRef(cpRaw)

	c1, err := ledger.NewSignedEvent(ledger.SignedEventInput{
		Author:     string(w1.ID),
		AuthorPub:  w1Pub,
		AuthorPriv: w1Priv,
		Seq:        1,
		Kind:       ledger.KindWitnessCheckpoint,
		Payload:    cpRaw,
		PayloadRef: cpRef,
		CreatedAt:  time.Now(),
	})
	if err != nil {
		t.Fatalf("new checkpoint 1: %v", err)
	}
	if _, err := ledgers[0].Apply(c1, &pol, nil); err != nil {
		t.Fatalf("apply checkpoint 1 on node0: %v", err)
	}

	c2, err := ledger.NewSignedEvent(ledger.SignedEventInput{
		Author:     string(w2.ID),
		AuthorPub:  w2Pub,
		AuthorPriv: w2Priv,
		Seq:        1,
		Kind:       ledger.KindWitnessCheckpoint,
		Payload:    cpRaw,
		PayloadRef: cpRef,
		CreatedAt:  time.Now(),
	})
	if err != nil {
		t.Fatalf("new checkpoint 2: %v", err)
	}
	if _, err := ledgers[0].Apply(c2, &pol, nil); err != nil {
		t.Fatalf("apply checkpoint 2 on node0: %v", err)
	}

	cm := ledger.EventsMessage{SchemaVersion: ledger.SyncSchemaV1, Events: []ledger.Event{c1, c2}}
	cpPt, err := encodeWire("events", "cp", ctxName, cm)
	if err != nil {
		t.Fatalf("encode checkpoint wire: %v", err)
	}
	for attempt := 0; attempt < 6; attempt++ {
		for to := 1; to < n; to++ {
			deliver(0, to, cpPt)
		}
		time.Sleep(10 * time.Millisecond)
	}

	deadline := time.NewTimer(4 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for {
		allCheckpointed := true
		for i := 0; i < n; i++ {
			if !ledgers[i].Have(c1.ID) || !ledgers[i].Have(c2.ID) {
				allCheckpointed = false
				break
			}
			if _, ok := ledgers[i].Checkpointed(ctxName, headsHash, &pol); !ok {
				allCheckpointed = false
				break
			}
		}
		if allCheckpointed {
			return
		}
		select {
		case <-deadline.C:
			miss := make([]int, 0, n)
			for i := 0; i < n; i++ {
				if _, ok := ledgers[i].Checkpointed(ctxName, headsHash, &pol); !ok {
					miss = append(miss, i)
				}
			}
			t.Fatalf("checkpoint quorum not reached on nodes: %v", miss)
		case <-tick.C:
		}
	}
}
