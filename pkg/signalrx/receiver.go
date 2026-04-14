package signalrx

import (
	"log"
	"time"
)

// Receiver mocks a one-way Signal attachment listener.
// In reality, this would bind to a local Signal-CLI daemon or libsignal-client via IPC/Sockets.
type Receiver struct {
	PollInterval time.Duration
	URIChan      chan string
}

func NewReceiver(pollInterval time.Duration) *Receiver {
	return &Receiver{
		PollInterval: pollInterval,
		URIChan:      make(chan string, 5),
	}
}

// Start polling for new bundles asynchronously
func (rx *Receiver) Start() {
	go func() {
		log.Println("signalrx: Started polling for encrypted bundles via Signal attachments")

		// Mock a received bundle after some time
		time.Sleep(15 * time.Second)

		log.Println("signalrx: Received new attachment: bundle.snb")

		rx.URIChan <- "snb://v2:REPLACE_WITH_BASE64URL_WRAPPER_JSON"
	}()
}
