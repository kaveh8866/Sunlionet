package mobilebridge

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	BLEMeshStateIdle               = "IDLE"
	BLEMeshStateAdvertisingVersion = "ADVERTISING_VERSION"
	BLEMeshStateDiscovering        = "DISCOVERING"
	BLEMeshStateConnectingGATT     = "CONNECTING_GATT"
	BLEMeshStateChunkTransfer      = "CHUNK_TRANSFER"
	BLEMeshStateSuspended          = "SUSPENDED"

	BLEAdvertPayloadSize = 22
	BLEChunkSize         = 160
	MaxBLEPayloadBytes   = 1 << 20
)

var (
	ErrBLEPayloadTooLarge = errors.New("ble mesh: payload too large")
	ErrBLEInvalidChunk    = errors.New("ble mesh: invalid chunk")
)

type BLEMeshConfig struct {
	AdvertiseMs int64 `json:"advertise_ms"`
	ScanMs      int64 `json:"scan_ms"`
	SuspendMs   int64 `json:"suspend_ms"`
}

type BLEMeshState struct {
	Mode             string `json:"mode"`
	ConfigVersionB64 string `json:"config_version_b64url"`
	PeerID           string `json:"peer_id,omitempty"`
	UpdatedAtUnix    int64  `json:"updated_at_unix"`
}

type BLETransferCheckpoint struct {
	TransferID     string `json:"transfer_id"`
	PayloadHashB64 string `json:"payload_hash_b64url"`
	TotalChunks    int    `json:"total_chunks"`
	ChunkSize      int    `json:"chunk_size"`
	Received       []bool `json:"received"`
	UpdatedAtUnix  int64  `json:"updated_at_unix"`
}

type BLEMeshEngine struct {
	mu         sync.Mutex
	state      BLEMeshState
	cfg        BLEMeshConfig
	checkpoint BLETransferCheckpoint
}

func NewBLEMeshEngine(configVersion []byte, cfg BLEMeshConfig) *BLEMeshEngine {
	if cfg.AdvertiseMs <= 0 {
		cfg.AdvertiseMs = 2500
	}
	if cfg.ScanMs <= 0 {
		cfg.ScanMs = 4500
	}
	if cfg.SuspendMs <= 0 {
		cfg.SuspendMs = 18_000
	}
	return &BLEMeshEngine{
		cfg: cfg,
		state: BLEMeshState{
			Mode:             BLEMeshStateIdle,
			ConfigVersionB64: base64.RawURLEncoding.EncodeToString(truncate(configVersion, 16)),
			UpdatedAtUnix:    time.Now().Unix(),
		},
	}
}

func (e *BLEMeshEngine) State() BLEMeshState {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.state
}

func (e *BLEMeshEngine) Next(now time.Time, peerSeen bool) BLEMeshState {
	e.mu.Lock()
	defer e.mu.Unlock()
	switch e.state.Mode {
	case BLEMeshStateIdle, "":
		e.state.Mode = BLEMeshStateAdvertisingVersion
	case BLEMeshStateAdvertisingVersion:
		e.state.Mode = BLEMeshStateDiscovering
	case BLEMeshStateDiscovering:
		if peerSeen {
			e.state.Mode = BLEMeshStateConnectingGATT
		} else {
			e.state.Mode = BLEMeshStateSuspended
		}
	case BLEMeshStateConnectingGATT:
		e.state.Mode = BLEMeshStateChunkTransfer
	case BLEMeshStateChunkTransfer:
		e.state.Mode = BLEMeshStateSuspended
	default:
		e.state.Mode = BLEMeshStateAdvertisingVersion
	}
	e.state.UpdatedAtUnix = now.Unix()
	return e.state
}

func BuildBLEAdvertPayload(secret []byte, nodeID []byte, configVersion []byte, nowUnix int64) ([]byte, error) {
	if len(secret) == 0 || len(nodeID) == 0 || len(configVersion) == 0 {
		return nil, fmt.Errorf("ble mesh: missing advert input")
	}
	epoch := uint32(nowUnix / 90)
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write(nodeID)
	var eb [4]byte
	binary.LittleEndian.PutUint32(eb[:], epoch)
	_, _ = mac.Write(eb[:])
	eph := mac.Sum(nil)

	ver := sha256.Sum256(configVersion)
	out := make([]byte, BLEAdvertPayloadSize)
	out[0] = 'S'
	out[1] = 'M'
	out[2] = 1
	out[3] = 0
	binary.LittleEndian.PutUint32(out[4:8], epoch)
	copy(out[8:16], eph[:8])
	copy(out[16:22], ver[:6])
	return out, nil
}

func ParseBLEAdvertPayload(raw []byte) (epoch uint32, peerID []byte, versionHash []byte, ok bool) {
	if len(raw) != BLEAdvertPayloadSize || raw[0] != 'S' || raw[1] != 'M' || raw[2] != 1 {
		return 0, nil, nil, false
	}
	epoch = binary.LittleEndian.Uint32(raw[4:8])
	peerID = append([]byte(nil), raw[8:16]...)
	versionHash = append([]byte(nil), raw[16:22]...)
	return epoch, peerID, versionHash, true
}

func NewBLETransferCheckpoint(payload []byte, chunkSize int) (BLETransferCheckpoint, error) {
	if len(payload) > MaxBLEPayloadBytes {
		return BLETransferCheckpoint{}, ErrBLEPayloadTooLarge
	}
	if chunkSize <= 0 {
		chunkSize = BLEChunkSize
	}
	total := (len(payload) + chunkSize - 1) / chunkSize
	if total == 0 {
		total = 1
	}
	sum := sha256.Sum256(payload)
	id := base64.RawURLEncoding.EncodeToString(sum[:12])
	return BLETransferCheckpoint{
		TransferID:     id,
		PayloadHashB64: base64.RawURLEncoding.EncodeToString(sum[:]),
		TotalChunks:    total,
		ChunkSize:      chunkSize,
		Received:       make([]bool, total),
		UpdatedAtUnix:  time.Now().Unix(),
	}, nil
}

func (c *BLETransferCheckpoint) MarkChunk(index int, chunk []byte) error {
	if c == nil || index < 0 || index >= c.TotalChunks || len(c.Received) != c.TotalChunks {
		return ErrBLEInvalidChunk
	}
	if len(chunk) > c.ChunkSize {
		return ErrBLEInvalidChunk
	}
	c.Received[index] = true
	c.UpdatedAtUnix = time.Now().Unix()
	return nil
}

func (c BLETransferCheckpoint) NextMissing() int {
	for i, ok := range c.Received {
		if !ok {
			return i
		}
	}
	return -1
}

func (c BLETransferCheckpoint) Complete() bool {
	return c.NextMissing() == -1
}

func SaveBLECheckpoint(path string, c BLETransferCheckpoint) error {
	raw, err := json.Marshal(c)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func LoadBLECheckpoint(path string) (BLETransferCheckpoint, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return BLETransferCheckpoint{}, err
	}
	var c BLETransferCheckpoint
	if err := json.Unmarshal(raw, &c); err != nil {
		return BLETransferCheckpoint{}, err
	}
	if c.TotalChunks <= 0 || len(c.Received) != c.TotalChunks || c.ChunkSize <= 0 {
		return BLETransferCheckpoint{}, ErrBLEInvalidChunk
	}
	return c, nil
}

func truncate(b []byte, n int) []byte {
	if len(b) <= n {
		return append([]byte(nil), b...)
	}
	return append([]byte(nil), b[:n]...)
}
