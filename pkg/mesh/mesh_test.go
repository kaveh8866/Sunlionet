package mesh

import (
	"testing"
	"time"
)

func TestEncryptAndDecryptOffer(t *testing.T) {
	// Initialize Alice
	alice, err := NewCrypto()
	if err != nil {
		t.Fatalf("Failed to initialize Alice: %v", err)
	}

	// Initialize Bob
	bob, err := NewCrypto()
	if err != nil {
		t.Fatalf("Failed to initialize Bob: %v", err)
	}

	// Alice creates an offer for Bob
	offer := ProxyOffer{
		Timestamp: time.Now().Unix(),
		Config:    "eyJ0eXBlIjogInNoYWRvd3RscyIsICJzZXJ2ZXIiOiAiMTkyLjE2OC40LjUiLCAicG9ydCI6IDg0NDN9",
		HopCount:  1,
	}

	// Alice encrypts offer with Bob's public key
	msg, err := alice.EncryptOffer(offer, bob.PublicKey())
	if err != nil {
		t.Fatalf("Failed to encrypt offer: %v", err)
	}

	got, err := bob.DecryptOffer(msg)
	if err != nil {
		t.Fatalf("Bob failed to decrypt Alice's message: %v", err)
	}
	if got.HopCount != offer.HopCount {
		t.Fatalf("expected hop=%d got=%d", offer.HopCount, got.HopCount)
	}
}

func TestHandleIncomingMessage_InvalidKey(t *testing.T) {
	alice, _ := NewCrypto()
	bob, _ := NewCrypto()
	eve, _ := NewCrypto()

	offer := ProxyOffer{Timestamp: time.Now().Unix(), HopCount: 1}

	// Alice encrypts for Bob
	msg, _ := alice.EncryptOffer(offer, bob.PublicKey())

	// Eve tries to decrypt (should fail)
	if _, err := eve.DecryptOffer(msg); err == nil {
		t.Fatalf("Eve should not be able to decrypt Alice's message to Bob")
	}
}

func TestForwardMessage_HopByHopForwarding(t *testing.T) {
	alice, _ := NewCrypto()
	relay, _ := NewCrypto()
	carol, _ := NewCrypto()

	offer := ProxyOffer{Timestamp: time.Now().Unix(), Config: "cfg", HopCount: 1}
	msgToRelay, err := alice.EncryptOffer(offer, relay.PublicKey())
	if err != nil {
		t.Fatalf("encrypt to relay: %v", err)
	}

	msgToCarol, err := relay.ForwardMessage(msgToRelay, carol.PublicKey())
	if err != nil {
		t.Fatalf("forward: %v", err)
	}

	got, err := carol.DecryptOffer(msgToCarol)
	if err != nil {
		t.Fatalf("carol decrypt: %v", err)
	}
	if got.HopCount != 2 {
		t.Fatalf("expected hop count to increment to 2, got %d", got.HopCount)
	}
}
