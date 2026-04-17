package sim

import (
	"context"

	"github.com/kaveh/shadownet-agent/pkg/orchestrator"
	"github.com/kaveh/shadownet-agent/pkg/runtimecfg"
)

type SimDetector struct {
	State orchestrator.NetworkState
	Err   error
}

func New(state orchestrator.NetworkState) *SimDetector {
	return &SimDetector{State: state}
}

func (d *SimDetector) Analyze(ctx context.Context) (orchestrator.NetworkState, error) {
	_ = ctx
	if d.Err != nil {
		return orchestrator.NetworkState{}, d.Err
	}
	return d.State, nil
}

func (d *SimDetector) RuntimeMode() runtimecfg.RuntimeMode {
	return runtimecfg.ModeSim
}
