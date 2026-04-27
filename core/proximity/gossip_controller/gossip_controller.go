package gossip_controller

import (
	"math/rand"
	"time"
)

type Decision struct {
	Forward bool
	Delay   time.Duration
}

type Controller struct {
	rnd *rand.Rand
}

func New(seed int64) *Controller {
	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	return &Controller{rnd: rand.New(rand.NewSource(seed))}
}

func (g *Controller) Decide(prob float64, maxDelay time.Duration) Decision {
	if prob <= 0 {
		return Decision{Forward: false}
	}
	if prob >= 1 {
		return Decision{Forward: true, Delay: g.jitter(maxDelay)}
	}
	if g.rnd.Float64() > prob {
		return Decision{Forward: false}
	}
	return Decision{Forward: true, Delay: g.jitter(maxDelay)}
}

func (g *Controller) jitter(maxDelay time.Duration) time.Duration {
	if maxDelay <= 0 {
		return 0
	}
	ns := g.rnd.Int63n(int64(maxDelay) + 1)
	return time.Duration(ns)
}
