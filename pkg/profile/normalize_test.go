package profile

import "testing"

func TestNormalizeForWire_UnsupportedFamily(t *testing.T) {
	_, err := NormalizeForWire(Profile{
		ID:     "p1",
		Family: Family("vless"),
		Endpoint: Endpoint{
			Host: "example.com",
			Port: 443,
		},
		Capabilities: Capabilities{
			Transport: "tcp",
		},
		Source: SourceInfo{
			Source:     "test",
			TrustLevel: 80,
		},
	}, 123)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestNormalizeForWire_ExpiredProfileRejected(t *testing.T) {
	_, err := NormalizeForWire(Profile{
		ID:     "p1",
		Family: FamilyDNS,
		Endpoint: Endpoint{
			Host: "example.com",
			Port: 53,
		},
		Capabilities: Capabilities{
			Transport: "udp",
		},
		Source: SourceInfo{
			Source:     "test",
			TrustLevel: 80,
		},
		CreatedAt: 100,
		ExpiresAt: 110,
	}, 200)
	if err == nil {
		t.Fatalf("expected error")
	}
}
