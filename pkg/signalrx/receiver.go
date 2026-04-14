package signalrx

import (
	"log"
	"time"

	"github.com/kaveh/shadownet-agent/pkg/bundle"
)

// Receiver mocks a one-way Signal attachment listener.
// In reality, this would bind to a local Signal-CLI daemon or libsignal-client via IPC/Sockets.
type Receiver struct {
	PollInterval time.Duration
	BundleChan   chan bundle.BundlePayload
}

func NewReceiver(pollInterval time.Duration) *Receiver {
	return &Receiver{
		PollInterval: pollInterval,
		BundleChan:   make(chan bundle.BundlePayload, 5),
	}
}

// Start polling for new bundles asynchronously
func (rx *Receiver) Start() {
	go func() {
		log.Println("signalrx: Started polling for encrypted bundles via Signal attachments")

		// Mock a received bundle after some time
		time.Sleep(15 * time.Second)

		log.Println("signalrx: Received new attachment: bundle.snb")

		// In a real system, we would:
		// 1. Verify Ed25519 signature
		// 2. Decrypt ChaCha20Poly1305 with device private key
		// 3. Unmarshal the bundle.BundlePayload

		// We push a mock payload to the channel
		rx.BundleChan <- bundle.BundlePayload{
			SchemaVersion:   1,
			MinAgentVersion: "1.0.0",
			Revocations:     []string{"reality_compromised_x"},
			// Profiles: ...
		}
	}()
}
