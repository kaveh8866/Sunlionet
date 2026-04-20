package real

import (
	"context"
	"errors"

	"github.com/kaveh/sunlionet-agent/pkg/runtimecfg"
)

var errUnavailable = errors.New("signalrx: no real receiver available")

type Receiver struct{}

func New() *Receiver {
	return &Receiver{}
}

func (r *Receiver) Receive(ctx context.Context) ([]byte, error) {
	_ = ctx
	return nil, errUnavailable
}

func (r *Receiver) RuntimeMode() runtimecfg.RuntimeMode {
	return runtimecfg.ModeReal
}
