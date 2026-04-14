package detector

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"time"

	"github.com/kaveh/shadownet-agent/pkg/profile"
)

// Worker orchestrates active and passive probes.
type Worker struct {
	EventChan chan Event
	Budget    ProbeBudget
}

type ProbeBudget struct {
	MaxActiveTCPPerHour int
	MaxActiveUDPPerHour int
	MaxActiveDNSPerHour int
}

func NewWorker(eventChan chan Event) *Worker {
	return &Worker{
		EventChan: eventChan,
		Budget: ProbeBudget{
			MaxActiveTCPPerHour: 10,
			MaxActiveUDPPerHour: 10,
			MaxActiveDNSPerHour: 20,
		},
	}
}

// Start begins continuous passive observation and jittered active probing.
func (w *Worker) Start(activeProfile profile.Profile) {
	log.Println("detector: Starting health observation and active probes")
	go w.runActiveProbes(activeProfile)
	go w.runPassiveObservation()
}

// runActiveProbes performs lightweight TCP/UDP/DNS checks at randomized intervals
func (w *Worker) runActiveProbes(p profile.Profile) {
	// Add initial jitter
	time.Sleep(time.Duration(rand.Intn(10)) * time.Second)

	for {
		// Mock TCP Connect probe against canary domain
		// E.g., check if port 443 is universally dropping or if it's just the proxy endpoint
		w.probeTCP("canary.example.com:443")

		// Based on profile capabilities, test the relevant protocol health
		if p.Capabilities.Transport == "udp" {
			w.probeUDP(p.Endpoint.Host, p.Endpoint.Port)
		} else {
			w.probeTCP(net.JoinHostPort(p.Endpoint.Host, fmt.Sprintf("%d", p.Endpoint.Port)))
		}

		// Wait before next probe (jittered to avoid DPI pattern recognition)
		sleepSec := 60 + rand.Intn(60) // 1-2 minutes
		time.Sleep(time.Duration(sleepSec) * time.Second)
	}
}

func (w *Worker) probeTCP(address string) {
	// Lightweight TCP Connect
	conn, err := net.DialTimeout("tcp", address, 3*time.Second)
	if err != nil {
		log.Printf("detector: Active TCP probe failed to %s: %v", address, err)
		// We could emit a HANDSHAKE_BURST_FAILURE or THROUGHPUT_COLLAPSE
		// depending on the exact error (e.g. connection refused vs connection reset by peer)
		w.EventChan <- Event{
			Type:      EventHandshakeBurstFailure,
			Severity:  SeverityMedium,
			Timestamp: time.Now(),
			Metadata:  map[string]string{"target": address, "error": err.Error()},
		}
		return
	}
	defer conn.Close()
	log.Printf("detector: Active TCP probe success to %s", address)
}

func (w *Worker) probeUDP(host string, port int) {
	address := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	// Lightweight UDP ping (e.g., QUIC stateless reset check)
	conn, err := net.DialTimeout("udp", address, 3*time.Second)
	if err != nil {
		log.Printf("detector: Active UDP probe failed to %s: %v", address, err)
		w.EventChan <- Event{
			Type:      EventUDPBlockSuspected,
			Severity:  SeverityHigh,
			Timestamp: time.Now(),
		}
		return
	}
	defer conn.Close()

	// Write a single byte to check reply ratio
	conn.Write([]byte{0x00})

	// Set read deadline
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 1)
	_, err = conn.Read(buf)
	if err != nil {
		log.Printf("detector: UDP probe sent but no reply from %s (Reply Ratio 0%%)", address)
		w.EventChan <- Event{
			Type:      EventUDPBlockSuspected,
			Severity:  SeverityHigh,
			Timestamp: time.Now(),
		}
		return
	}

	log.Printf("detector: Active UDP probe success to %s", address)
}

func (w *Worker) runPassiveObservation() {
	// Mock: This would hook into sing-box's internal stats/log stream or OS TUN interface metrics.
	// We monitor exit codes, connection resets, and TLS alerts without sending our own packets.
	for {
		time.Sleep(10 * time.Minute)
		// Emit recovery if no errors seen
		w.EventChan <- Event{
			Type:      EventRecoveryConfirmed,
			Severity:  SeverityLow,
			Timestamp: time.Now(),
		}
	}
}
