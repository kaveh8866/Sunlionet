package security

import (
	"time"

	"github.com/kaveh/sunlionet-agent/core/proximity/message_router"
	"github.com/kaveh/sunlionet-agent/core/transport/transport_selector"
)

type Mode uint8

const (
	ModeStealth Mode = iota + 1
	ModeBalanced
	ModeResilience
)

type Policy struct {
	Mode      Mode
	Multipath bool

	CoverTraffic bool

	MaxPayloadBytes int
}

func (p Policy) TransportProfile() transport_selector.Profile {
	switch p.Mode {
	case ModeStealth:
		return transport_selector.ProfileStealth
	case ModeResilience:
		return transport_selector.ProfileBalanced
	default:
		return transport_selector.ProfileBalanced
	}
}

func (p Policy) MultipathEnabled() bool {
	if p.Mode == ModeResilience {
		return true
	}
	return p.Multipath
}

func (p Policy) PreferenceForPayload(payloadBytes int) transport_selector.Preference {
	switch p.Mode {
	case ModeStealth:
		return transport_selector.PreferencePreferBLE
	case ModeResilience:
		if payloadBytes <= 200 {
			return transport_selector.PreferenceAny
		}
		return transport_selector.PreferencePreferWiFi
	default:
		return transport_selector.PreferenceAny
	}
}

func (p Policy) ApplyRouterDefaults(opts message_router.Options) message_router.Options {
	if p.MaxPayloadBytes > 0 {
		opts.MaxPayloadBytes = p.MaxPayloadBytes
	}
	if p.CoverTraffic {
		if opts.CoverTrafficEvery == 0 {
			switch p.Mode {
			case ModeStealth:
				opts.CoverTrafficEvery = 600 * time.Millisecond
				opts.CoverTrafficSize = 16
			case ModeResilience:
				opts.CoverTrafficEvery = 350 * time.Millisecond
				opts.CoverTrafficSize = 24
			default:
				opts.CoverTrafficEvery = 0
			}
		}
	} else {
		opts.CoverTrafficEvery = 0
	}
	return opts
}
