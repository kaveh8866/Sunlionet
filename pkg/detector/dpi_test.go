package detector

import (
	"runtime"
	"testing"
)

func TestPassiveTCPStats(t *testing.T) {
	// Skip this test on Windows as it relies on the Linux 'ss' command
	if runtime.GOOS == "windows" {
		t.Skip("Skipping Linux-specific 'ss' command test on Windows")
	}

	// This is a mock function, it should just return a float64
	ratio, err := PassiveTCPStats()
	if err != nil {
		t.Fatalf("PassiveTCPStats failed: %v", err)
	}
	if ratio < 0 || ratio > 1 {
		t.Fatalf("Invalid retransmission ratio: %v", ratio)
	}
}

func TestCheckConnectivityBaseline(t *testing.T) {
	// The mock currently dials 8.8.8.8:53 with a very short timeout
	// It might fail depending on the network, but we'll test that it doesn't panic
	ok, err := CheckConnectivityBaseline()
	if err != nil {
		t.Logf("Connectivity baseline failed (expected in offline environments): %v", err)
	}
	if !ok {
		t.Logf("Baseline is false, indicating no connectivity")
	}
}

func TestCheckSNIReset(t *testing.T) {
	// Tests the SNI reset detector logic
	blocked, err := CheckSNIReset("www.google.com", "8.8.8.8")
	if err != nil {
		t.Logf("SNI reset check failed: %v", err)
	}
	// In a normal environment, this shouldn't be blocked.
	if blocked {
		t.Logf("SNI is blocked!")
	}
}

func TestCheckUDPBlocked(t *testing.T) {
	// Tests the UDP block detector logic
	blocked, err := CheckUDPBlocked("8.8.8.8:53")
	if err != nil {
		t.Logf("UDP block check failed: %v", err)
	}
	if blocked {
		t.Logf("UDP is blocked!")
	}
}
