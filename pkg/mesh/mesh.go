package mesh

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"log"
	"time"

	"golang.org/x/crypto/nacl/box"
)

var (
	errDecrypt = errors.New("mesh decrypt failed")
	errDecode  = errors.New("mesh payload decode failed")
)

// MeshMessage represents an encrypted payload exchanged over Bluetooth
type MeshMessage struct {
	Nonce      [24]byte `json:"nonce"`
	Ciphertext []byte   `json:"ciphertext"`
	SenderPub  [32]byte `json:"sender_pub"`
}

// ProxyOffer is the plaintext payload inside a MeshMessage
type ProxyOffer struct {
	Timestamp int64  `json:"timestamp"`
	Config    string `json:"config_base64"` // Anonymized sing-box outbound config
	HopCount  int    `json:"hop_count"`
}

// MeshManager handles Bluetooth scanning and encrypted proxy exchange
type MeshManager struct {
	privKey [32]byte
	pubKey  [32]byte
	active  bool
}

func NewMeshManager() (*MeshManager, error) {
	pub, priv, err := box.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	return &MeshManager{
		privKey: *priv,
		pubKey:  *pub,
		active:  false,
	}, nil
}

// Start scanning and broadcasting in Total Blackout mode
func (m *MeshManager) Start() {
	m.active = true
	log.Println("[MESH] Total blackout detected. Starting Bluetooth Mesh mode.")

	// In a real implementation on Linux, this would use github.com/muka/go-bluetooth
	// On Android, this uses Android Nearby Connections API via JNI/bindings.
	go m.mockBluetoothScanner()
	go m.mockBluetoothBroadcaster()
}

func (m *MeshManager) Stop() {
	m.active = false
	log.Println("[MESH] Internet restored. Stopping Bluetooth Mesh.")
}

// mockBluetoothScanner simulates finding a peer and receiving a proxy offer
func (m *MeshManager) mockBluetoothScanner() {
	for m.active {
		time.Sleep(15 * time.Second)
		log.Println("[MESH] Scanning for ShadowNet peers...")

		// Simulate receiving a valid proxy offer from a peer
		peerPub, _, _ := box.GenerateKey(rand.Reader)
		offer := ProxyOffer{
			Timestamp: time.Now().Unix(),
			Config:    "eyJ0eXBlIjogInNoYWRvd3RscyIsICJzZXJ2ZXIiOiAiMTkyLjE2OC40LjUiLCAicG9ydCI6IDg0NDN9", // Mock base64 JSON
			HopCount:  1,
		}

		msg, err := m.EncryptOffer(offer, peerPub)
		if err == nil {
			log.Printf("[MESH] Found peer %x! Exchanging encrypted configs...", peerPub[:4])
			m.handleIncomingMessage(msg)
		}
	}
}

// mockBluetoothBroadcaster simulates broadcasting our healthy configs to peers
func (m *MeshManager) mockBluetoothBroadcaster() {
	for m.active {
		time.Sleep(20 * time.Second)
		log.Println("[MESH] Broadcasting local healthy proxy offers (anonymized)...")
		// Implementation would broadcast GATT advertisement with service UUID
	}
}

// EncryptOffer creates an anonymous, encrypted X25519 message
func (m *MeshManager) EncryptOffer(offer ProxyOffer, recipientPub *[32]byte) (MeshMessage, error) {
	plaintext, err := json.Marshal(offer)
	if err != nil {
		return MeshMessage{}, err
	}

	var nonce [24]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return MeshMessage{}, err
	}

	ciphertext := box.Seal(nil, plaintext, &nonce, recipientPub, &m.privKey)

	return MeshMessage{
		Nonce:      nonce,
		Ciphertext: ciphertext,
		SenderPub:  m.pubKey,
	}, nil
}

// DecryptOffer decapsulates a mesh message and returns the plaintext offer.
func (m *MeshManager) DecryptOffer(msg MeshMessage) (ProxyOffer, error) {
	plaintext, ok := box.Open(nil, msg.Ciphertext, &msg.Nonce, &msg.SenderPub, &m.privKey)
	if !ok {
		return ProxyOffer{}, errDecrypt
	}

	var offer ProxyOffer
	if err := json.Unmarshal(plaintext, &offer); err != nil {
		return ProxyOffer{}, errDecode
	}

	return offer, nil
}

// ForwardMessage decrypts, increments hop count, and re-encrypts for the next hop.
func (m *MeshManager) ForwardMessage(msg MeshMessage, nextHopPub *[32]byte) (MeshMessage, error) {
	offer, err := m.DecryptOffer(msg)
	if err != nil {
		return MeshMessage{}, err
	}
	offer.HopCount++
	return m.EncryptOffer(offer, nextHopPub)
}

// handleIncomingMessage decrypts and parses peer offers
func (m *MeshManager) handleIncomingMessage(msg MeshMessage) {
	plaintext, ok := box.Open(nil, msg.Ciphertext, &msg.Nonce, &msg.SenderPub, &m.privKey)
	if !ok {
		log.Println("[MESH] Failed to decrypt incoming mesh message. Ignoring.")
		return
	}

	var offer ProxyOffer
	if err := json.Unmarshal(plaintext, &offer); err != nil {
		log.Println("[MESH] Invalid proxy offer format.")
		return
	}

	log.Printf("[MESH] Successfully decrypted proxy offer. Hop Count: %d", offer.HopCount)
	// Here, the agent would add this to the Candidate list in policy.RotationManager
	// and run sing-box check on it.
}
