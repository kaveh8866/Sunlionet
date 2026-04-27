package sparse_mesh_routing

import "time"

type CourierPolicy struct {
	ContactMin time.Duration
	HoldTTL    time.Duration
}

func DefaultCourierPolicy() CourierPolicy {
	return CourierPolicy{
		ContactMin: 150 * time.Millisecond,
		HoldTTL:    10 * time.Minute,
	}
}

type Threat struct {
	Transport    string
	Name         string
	Vector       string
	Mitigation   string
	ResidualRisk string
}

func ThreatModel() []Threat {
	return []Threat{
		{
			Transport:    "lora",
			Name:         "Jamming",
			Vector:       "RF interference prevents reception",
			Mitigation:   "Adaptive retry/backoff, multi-path redundancy, store-and-forward",
			ResidualRisk: "Sustained wideband jamming blocks the channel",
		},
		{
			Transport:    "lora",
			Name:         "Traffic analysis",
			Vector:       "Packet timing and volume reveal activity",
			Mitigation:   "Optional cover traffic and batching, constant-size frames where possible",
			ResidualRisk: "Active observers can still infer coarse patterns",
		},
		{
			Transport:    "lora",
			Name:         "Gateway targeting",
			Vector:       "Physical or legal pressure on known gateways",
			Mitigation:   "Decentralized relays, rotating bridge roles, minimize metadata",
			ResidualRisk: "High-profile fixed gateways remain vulnerable",
		},
		{
			Transport:    "courier",
			Name:         "Delayed delivery abuse",
			Vector:       "Adversary delays bundles to reduce utility",
			Mitigation:   "TTL enforcement, expiration windows, opportunistic multi-carrier spread",
			ResidualRisk: "Delays are inherent to DTN-style networks",
		},
		{
			Transport:    "courier",
			Name:         "Bundle injection",
			Vector:       "Malicious nodes inject junk bundles to exhaust storage",
			Mitigation:   "Rate limits, cache caps, replay protection, local scoring/quarantine",
			ResidualRisk: "Resource attacks remain possible under extreme conditions",
		},
	}
}

type SideChannelStudy struct {
	Name          string
	RangeClass    string
	Bandwidth     string
	Detectability string
	Notes         string
}

func SideChannelResearch() []SideChannelStudy {
	return []SideChannelStudy{
		{
			Name:          "Ultrasonic",
			RangeClass:    "short",
			Bandwidth:     "very low",
			Detectability: "moderate",
			Notes:         "Potential fallback for small rendezvous tokens; unreliable across devices",
		},
		{
			Name:          "Light/QR burst",
			RangeClass:    "line-of-sight",
			Bandwidth:     "low to medium",
			Detectability: "high",
			Notes:         "Good for manual transfer of encrypted bundles; requires user action",
		},
		{
			Name:          "Acoustic (audible)",
			RangeClass:    "short",
			Bandwidth:     "very low",
			Detectability: "high",
			Notes:         "Emergency-only; high exposure and environmental sensitivity",
		},
	}
}
