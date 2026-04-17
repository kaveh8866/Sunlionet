package real

import (
	"context"
	"errors"

	"github.com/kaveh/shadownet-agent/pkg/runtimecfg"
)

var errUnavailable = errors.New("mesh: no real mesh implementation available")

type RealMesh struct{}

func New() *RealMesh {
	return &RealMesh{}
}

func (m *RealMesh) Broadcast(data []byte) error {
	_ = data
	return errUnavailable
}

func (m *RealMesh) Receive(ctx context.Context) ([]byte, error) {
	_ = ctx
	return nil, errUnavailable
}

func (m *RealMesh) RuntimeMode() runtimecfg.RuntimeMode {
	return runtimecfg.ModeReal
}
