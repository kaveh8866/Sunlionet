package identity

import "testing"

func TestDeviceJoinRequestEncodeDecodeApprove(t *testing.T) {
	p, err := NewPersona()
	if err != nil {
		t.Fatalf("NewPersona: %v", err)
	}
	d, err := NewDevice(p.ID, "phone")
	if err != nil {
		t.Fatalf("NewDevice: %v", err)
	}

	req, err := NewDeviceJoinRequest(p.ID, d.SignPubB64)
	if err != nil {
		t.Fatalf("NewDeviceJoinRequest: %v", err)
	}
	enc, err := req.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	dec, err := DecodeDeviceJoinRequest(enc)
	if err != nil {
		t.Fatalf("DecodeDeviceJoinRequest: %v", err)
	}
	if dec.PersonaID != p.ID {
		t.Fatalf("persona mismatch")
	}

	pkg, err := ApproveDeviceJoinRequest(p, dec)
	if err != nil {
		t.Fatalf("ApproveDeviceJoinRequest: %v", err)
	}
	if err := pkg.Validate(p.SignPubB64); err != nil {
		t.Fatalf("Validate join package: %v", err)
	}
}
