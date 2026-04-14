package mesh

import (
	"testing"
	"time"

	"golang.org/x/crypto/nacl/box"
)

func TestEncryptAndDecryptOffer(t *testing.T) {
	// Initialize Alice
	alice, err := NewMeshManager()
	if err != nil {
		t.Fatalf("Failed to initialize Alice: %v", err)
	}

	// Initialize Bob
	bob, err := NewMeshManager()
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
	msg, err := alice.EncryptOffer(offer, &bob.pubKey)
	if err != nil {
		t.Fatalf("Failed to encrypt offer: %v", err)
	}

	// Bob receives the message and decrypts it
	plaintext, ok := box.Open(nil, msg.Ciphertext, &msg.Nonce, &msg.SenderPub, &bob.privKey)
	if !ok {
		t.Fatalf("Bob failed to decrypt Alice's message")
	}

	if len(plaintext) == 0 {
		t.Fatalf("Decrypted payload is empty")
	}
}

func TestHandleIncomingMessage_InvalidKey(t *testing.T) {
	alice, _ := NewMeshManager()
	bob, _ := NewMeshManager()
	eve, _ := NewMeshManager()

	offer := ProxyOffer{Timestamp: time.Now().Unix(), HopCount: 1}

	// Alice encrypts for Bob
	msg, _ := alice.EncryptOffer(offer, &bob.pubKey)

	// Eve tries to decrypt (should fail gracefully)
	// handleIncomingMessage handles it internally without panic, but we can verify box.Open fails
	_, ok := box.Open(nil, msg.Ciphertext, &msg.Nonce, &msg.SenderPub, &eve.privKey)
	if ok {
		t.Fatalf("Eve should not be able to decrypt Alice's message to Bob")
	}
}
