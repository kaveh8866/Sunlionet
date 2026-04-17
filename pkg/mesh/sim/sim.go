package sim

import (
	"context"
	"errors"

	"github.com/kaveh/shadownet-agent/pkg/runtimecfg"
)

var errQueueFull = errors.New("mesh: queue full")

type SimMesh struct {
	ch chan []byte
}

func New(buffer int) *SimMesh {
	if buffer <= 0 {
		buffer = 16
	}
	return &SimMesh{ch: make(chan []byte, buffer)}
}

func (m *SimMesh) Broadcast(data []byte) error {
	cp := append([]byte(nil), data...)
	select {
	case m.ch <- cp:
		return nil
	default:
		return errQueueFull
	}
}

func (m *SimMesh) Receive(ctx context.Context) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case b := <-m.ch:
		return b, nil
	}
}

func (m *SimMesh) RuntimeMode() runtimecfg.RuntimeMode {
	return runtimecfg.ModeSim
}
