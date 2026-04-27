package transport_selector

type Profile uint8

const (
	ProfileStealth Profile = iota + 1
	ProfileBalanced
	ProfilePerformance
)

type Preference uint8

const (
	PreferenceAny Preference = iota + 1
	PreferencePreferBLE
	PreferencePreferWiFi
	PreferencePreferLoRa
)

type Candidate interface {
	Name() string
	Available() bool
}

type Selector struct {
	Profile Profile
}

type Decision struct {
	Names []string
}

func (s Selector) Decide(candidates []Candidate, pref Preference, payloadBytes int, multipath bool) Decision {
	available := make([]Candidate, 0, len(candidates))
	for i := range candidates {
		if candidates[i] != nil && candidates[i].Available() {
			available = append(available, candidates[i])
		}
	}
	if len(available) == 0 {
		return Decision{}
	}

	ble := make([]string, 0, 1)
	wifi := make([]string, 0, 1)
	lora := make([]string, 0, 1)
	for i := range available {
		switch available[i].Name() {
		case "ble":
			ble = append(ble, "ble")
		case "wifi_direct":
			wifi = append(wifi, "wifi_direct")
		case "lora":
			lora = append(lora, "lora")
		}
	}

	switch pref {
	case PreferencePreferBLE:
		if len(ble) > 0 {
			return Decision{Names: ble}
		}
		if len(wifi) > 0 {
			return Decision{Names: wifi}
		}
		if len(lora) > 0 {
			return Decision{Names: lora}
		}
	case PreferencePreferWiFi:
		if len(wifi) > 0 {
			return Decision{Names: wifi}
		}
		if len(ble) > 0 {
			return Decision{Names: ble}
		}
		if len(lora) > 0 {
			return Decision{Names: lora}
		}
	case PreferencePreferLoRa:
		if len(lora) > 0 {
			return Decision{Names: lora}
		}
		if len(ble) > 0 {
			return Decision{Names: ble}
		}
		if len(wifi) > 0 {
			return Decision{Names: wifi}
		}
	}

	switch s.Profile {
	case ProfileStealth:
		if multipath && len(ble) > 0 && len(lora) > 0 && payloadBytes <= 200 {
			return Decision{Names: []string{"ble", "lora"}}
		}
		if len(ble) > 0 {
			return Decision{Names: ble}
		}
		if len(wifi) > 0 {
			return Decision{Names: wifi}
		}
		return Decision{Names: lora}
	case ProfilePerformance:
		if multipath && len(wifi) > 0 && len(lora) > 0 && payloadBytes <= 200 {
			return Decision{Names: []string{"wifi_direct", "lora"}}
		}
		if len(wifi) > 0 {
			return Decision{Names: wifi}
		}
		if multipath && len(ble) > 0 && len(lora) > 0 && payloadBytes <= 200 {
			return Decision{Names: []string{"ble", "lora"}}
		}
		if len(ble) > 0 {
			return Decision{Names: ble}
		}
		return Decision{Names: lora}
	default:
		if multipath && len(ble) > 0 && len(wifi) > 0 {
			return Decision{Names: []string{"ble", "wifi_direct"}}
		}
		if multipath && len(wifi) > 0 && len(lora) > 0 && payloadBytes <= 200 {
			return Decision{Names: []string{"wifi_direct", "lora"}}
		}
		if multipath && len(ble) > 0 && len(lora) > 0 && payloadBytes <= 200 {
			return Decision{Names: []string{"ble", "lora"}}
		}
		if payloadBytes > 800 && len(wifi) > 0 {
			return Decision{Names: wifi}
		}
		if len(ble) > 0 {
			return Decision{Names: ble}
		}
		if len(wifi) > 0 {
			return Decision{Names: wifi}
		}
		if payloadBytes <= 200 && len(lora) > 0 {
			return Decision{Names: lora}
		}
		return Decision{Names: lora}
	}
}
