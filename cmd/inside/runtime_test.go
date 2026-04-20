//go:build !daemon
// +build !daemon

package main

import (
	"testing"

	"github.com/kaveh/sunlionet-agent/pkg/runtimecfg"
)

func TestBuildRuntime_ModeSim_UsesSimulationImplementations(t *testing.T) {
	rt := buildRuntime(runtimecfg.RuntimeConfig{Mode: runtimecfg.ModeSim})
	if rt.Detector.RuntimeMode() != runtimecfg.ModeSim {
		t.Fatalf("expected detector sim mode")
	}
	if rt.Mesh.RuntimeMode() != runtimecfg.ModeSim {
		t.Fatalf("expected mesh sim mode")
	}
	if rt.Signal.RuntimeMode() != runtimecfg.ModeSim {
		t.Fatalf("expected signalrx sim mode")
	}
}

func TestBuildRuntime_ModeReal_UsesRealImplementations(t *testing.T) {
	rt := buildRuntime(runtimecfg.RuntimeConfig{Mode: runtimecfg.ModeReal})
	if rt.Detector.RuntimeMode() != runtimecfg.ModeReal {
		t.Fatalf("expected detector real mode")
	}
	if rt.Mesh.RuntimeMode() != runtimecfg.ModeReal {
		t.Fatalf("expected mesh real mode")
	}
	if rt.Signal.RuntimeMode() != runtimecfg.ModeReal {
		t.Fatalf("expected signalrx real mode")
	}
}
