package sim

import (
	"context"

	"github.com/kaveh/shadownet-agent/pkg/runtimecfg"
)

type Receiver struct {
	ch chan []byte
}

func New(buffer int) *Receiver {
	if buffer <= 0 {
		buffer = 8
	}
	return &Receiver{ch: make(chan []byte, buffer)}
}

func (r *Receiver) Push(payload []byte) bool {
	cp := append([]byte(nil), payload...)
	select {
	case r.ch <- cp:
		return true
	default:
		return false
	}
}

func (r *Receiver) Receive(ctx context.Context) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case b := <-r.ch:
		return b, nil
	}
}

func (r *Receiver) RuntimeMode() runtimecfg.RuntimeMode {
	return runtimecfg.ModeSim
}
