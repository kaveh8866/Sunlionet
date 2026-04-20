package detector

import (
	"context"

	"github.com/kaveh/sunlionet-agent/pkg/orchestrator"
	"github.com/kaveh/sunlionet-agent/pkg/runtimecfg"
)

type Detector interface {
	Analyze(ctx context.Context) (orchestrator.NetworkState, error)
	RuntimeMode() runtimecfg.RuntimeMode
}
