package signalrx

import (
	"context"

	"github.com/kaveh/shadownet-agent/pkg/runtimecfg"
)

type SignalReceiver interface {
	Receive(ctx context.Context) ([]byte, error)
	RuntimeMode() runtimecfg.RuntimeMode
}
