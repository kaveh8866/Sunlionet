package identity

import (
	"strings"
	"testing"

	"filippo.io/age"
)

func TestDeviceJoinRequestEncodeDecodeApprove(t *testing.T) {
	p, err := NewPersona()
	if err != nil {
		t.Fatalf("NewPersona: %v", err)
	}
	d, err := NewDevice(p.ID, "phone")
	if err != nil {
		t.Fatalf("NewDevice: %v", err)
	}

	ageID, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("GenerateX25519Identity: %v", err)
	}
	ageRecipient := ageID.Recipient().String()

	req, err := NewDeviceJoinRequestForDevice(p.ID, d.DeviceID, d.SignPubB64, ageRecipient)
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
	link, err := NewDeviceLinkBundle(p, dec, pkg)
	if err != nil {
		t.Fatalf("NewDeviceLinkBundle: %v", err)
	}
	pkgEnc, err := EncryptDeviceLinkBundle(p, ageRecipient, link)
	if err != nil {
		t.Fatalf("EncryptDeviceLinkBundle: %v", err)
	}
	linkDec, err := DecryptDeviceLink(pkgEnc, ageID.String())
	if err != nil {
		t.Fatalf("DecryptDeviceLink: %v", err)
	}
	st := NewState()
	st.Devices = append(st.Devices, *d)
	if err := UpsertDeviceFromJoinPackage(st, p.SignPubB64, &linkDec.JoinPackage); err != nil {
		t.Fatalf("UpsertDeviceFromJoinPackage: %v", err)
	}
	if len(st.Devices) != 1 {
		t.Fatalf("expected 1 device, got %d", len(st.Devices))
	}
	if st.Devices[0].Trust != DeviceTrusted {
		t.Fatalf("expected trusted device")
	}
	st.Personas = append(st.Personas, *p)
}

func TestDeviceLinkBundleRejectsTooLongLifetime(t *testing.T) {
	p, err := NewPersona()
	if err != nil {
		t.Fatalf("NewPersona: %v", err)
	}
	d, err := NewDevice(p.ID, "phone")
	if err != nil {
		t.Fatalf("NewDevice: %v", err)
	}
	req, err := NewDeviceJoinRequestForDevice(p.ID, d.DeviceID, d.SignPubB64, "")
	if err != nil {
		t.Fatalf("NewDeviceJoinRequest: %v", err)
	}
	pkg, err := ApproveDeviceJoinRequest(p, req)
	if err != nil {
		t.Fatalf("ApproveDeviceJoinRequest: %v", err)
	}
	link, err := NewDeviceLinkBundle(p, req, pkg)
	if err != nil {
		t.Fatalf("NewDeviceLinkBundle: %v", err)
	}
	link.ExpiresAt = link.CreatedAt + (60 * 60)
	enc, err := link.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if _, err := DecodeDeviceLinkBundle(enc); err == nil {
		t.Fatalf("expected DecodeDeviceLinkBundle to fail")
	}
}

func TestDeviceLinkSAS_EncryptedBundle(t *testing.T) {
	p, err := NewPersona()
	if err != nil {
		t.Fatalf("NewPersona: %v", err)
	}
	d, err := NewDevice(p.ID, "phone")
	if err != nil {
		t.Fatalf("NewDevice: %v", err)
	}
	ageID, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("GenerateX25519Identity: %v", err)
	}
	ageRecipient := ageID.Recipient().String()

	req, err := NewDeviceJoinRequestForDevice(p.ID, d.DeviceID, d.SignPubB64, ageRecipient)
	if err != nil {
		t.Fatalf("NewDeviceJoinRequest: %v", err)
	}
	pkg, err := ApproveDeviceJoinRequest(p, req)
	if err != nil {
		t.Fatalf("ApproveDeviceJoinRequest: %v", err)
	}
	link, err := NewDeviceLinkBundle(p, req, pkg)
	if err != nil {
		t.Fatalf("NewDeviceLinkBundle: %v", err)
	}
	pkgEnc, err := EncryptDeviceLinkBundle(p, ageRecipient, link)
	if err != nil {
		t.Fatalf("EncryptDeviceLinkBundle: %v", err)
	}

	sas1, err := DeviceLinkSAS(pkgEnc)
	if err != nil {
		t.Fatalf("DeviceLinkSAS: %v", err)
	}
	sas2, err := DeviceLinkSAS(pkgEnc)
	if err != nil {
		t.Fatalf("DeviceLinkSAS(2): %v", err)
	}
	if sas1 != sas2 {
		t.Fatalf("expected sas stable")
	}
	if strings.Count(sas1, "-") != 2 || len(sas1) != len("0000-0000-0000") {
		t.Fatalf("unexpected sas format: %q", sas1)
	}

	if _, err := DecryptDeviceLink(pkgEnc, ageID.String()); err != nil {
		t.Fatalf("DecryptDeviceLink: %v", err)
	}
}
