package runtimecfg

import (
	"fmt"
	"strings"
)

type RuntimeMode string

const (
	ModeReal RuntimeMode = "real"
	ModeSim  RuntimeMode = "simulation"
)

type RuntimeConfig struct {
	Mode RuntimeMode
}

func ParseRuntimeMode(raw string) (RuntimeMode, error) {
	s := strings.TrimSpace(strings.ToLower(raw))
	if s == "" {
		return ModeReal, nil
	}
	switch RuntimeMode(s) {
	case ModeReal, ModeSim:
		return RuntimeMode(s), nil
	default:
		return "", fmt.Errorf("invalid mode %q (expected %q or %q)", raw, ModeReal, ModeSim)
	}
}
