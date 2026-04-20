package ledger

import (
	"crypto/ed25519"
	"errors"
	"time"
)

type Status string

const (
	StatusAccepted     Status = "accepted"
	StatusConfirmed    Status = "confirmed"
	StatusCheckpointed Status = "checkpointed"
)

type TrustPolicy struct {
	Witnesses        map[string]map[string]int
	Thresholds       map[string]int
	DefaultThreshold int
}

func (t TrustPolicy) Weight(context string, authorKeyB64 string) int {
	if t.Witnesses == nil {
		return 0
	}
	m := t.Witnesses[context]
	if m == nil {
		return 0
	}
	return m[authorKeyB64]
}

func (t TrustPolicy) Threshold(context string) int {
	if t.Thresholds != nil {
		if v, ok := t.Thresholds[context]; ok && v > 0 {
			return v
		}
	}
	if t.DefaultThreshold > 0 {
		return t.DefaultThreshold
	}
	return 0
}

type Policy struct {
	AllowUnknownKinds bool

	MaxClockSkew time.Duration
	MaxEventAge  time.Duration

	RequireKnownPrev bool

	RetentionByKind map[string]time.Duration

	Trust TrustPolicy
}

type Observer struct {
	Author     string
	AuthorPub  ed25519.PublicKey
	AuthorPriv ed25519.PrivateKey
}

func (o Observer) Valid() error {
	if o.Author == "" {
		return errors.New("ledger: observer author required")
	}
	if len(o.AuthorPub) != ed25519.PublicKeySize {
		return errors.New("ledger: observer public key invalid")
	}
	if len(o.AuthorPriv) != ed25519.PrivateKeySize {
		return errors.New("ledger: observer keys required")
	}
	return nil
}

func DefaultPolicy() Policy {
	return Policy{
		AllowUnknownKinds: true,
		MaxClockSkew:      10 * time.Minute,
		MaxEventAge:       0,
		RequireKnownPrev:  false,
		RetentionByKind:   map[string]time.Duration{},
		Trust: TrustPolicy{
			Witnesses:        map[string]map[string]int{},
			Thresholds:       map[string]int{},
			DefaultThreshold: 0,
		},
	}
}

func ProductionPolicy() Policy {
	p := DefaultPolicy()
	p.RequireKnownPrev = true
	p.RetentionByKind = map[string]time.Duration{
		KindWitnessAttest:      30 * 24 * time.Hour,
		KindWitnessCheckpoint:  90 * 24 * time.Hour,
		KindMisbehaviorEquivoc: 180 * 24 * time.Hour,
		KindMisbehaviorReplay:  30 * 24 * time.Hour,
		KindSyncSummary:        7 * 24 * time.Hour,
	}
	return p
}
