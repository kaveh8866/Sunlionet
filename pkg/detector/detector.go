package detector

import (
	"context"

	"github.com/kaveh/shadownet-agent/pkg/orchestrator"
	"github.com/kaveh/shadownet-agent/pkg/runtimecfg"
)

type Detector interface {
	Analyze(ctx context.Context) (orchestrator.NetworkState, error)
	RuntimeMode() runtimecfg.RuntimeMode
}
