package routing_engine

import (
	"time"

	"github.com/kaveh/sunlionet-agent/core/proximity/identity_manager"
)

type NeighborScorer interface {
	Score(id identity_manager.NodeID, now time.Time) float64
}

type Input struct {
	From identity_manager.NodeID

	Hop         uint8
	MaxHop      uint8
	ExpiresAt   time.Time
	Now         time.Time
	HasFrom     bool
	RateLimited bool
}

type Engine struct {
	BaseProb     float64
	MaxJitter    time.Duration
	MinTTLRemain time.Duration
}

func New() *Engine {
	return &Engine{
		BaseProb:     0.75,
		MaxJitter:    650 * time.Millisecond,
		MinTTLRemain: 3 * time.Second,
	}
}

func (e *Engine) Probability(in Input, scorer NeighborScorer) float64 {
	if !in.Now.Before(in.ExpiresAt) {
		return 0
	}
	ttlRemain := in.ExpiresAt.Sub(in.Now)
	if ttlRemain < e.MinTTLRemain {
		return 0.10
	}
	p := e.BaseProb
	if in.MaxHop > 0 {
		h := float64(in.Hop) / float64(in.MaxHop)
		if h > 1 {
			h = 1
		}
		p *= (1.0 - 0.55*h)
	}
	if in.HasFrom && scorer != nil {
		score := scorer.Score(in.From, in.Now)
		p *= 0.05 + 0.95*score
	} else {
		p *= 0.6
	}
	if in.RateLimited {
		p *= 0.2
	}
	if p < 0.0 {
		return 0
	}
	if p > 1.0 {
		return 1
	}
	return p
}

func (e *Engine) JitterMax() time.Duration {
	return e.MaxJitter
}
