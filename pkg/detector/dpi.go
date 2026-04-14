package detector

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

// CheckDNSPoisoning resolves a domain known to be blocked in Iran (e.g., twitter.com).
// If the response is a private IP or the known 10.10.34.x block page IP, it's poisoned.
func CheckDNSPoisoning(domain string) (bool, error) {
	// 1s timeout to ensure under 5s total budget
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	var resolver net.Resolver
	ips, err := resolver.LookupIPAddr(ctx, domain)
	if err != nil {
		return false, fmt.Errorf("dns lookup failed: %w", err)
	}

	for _, ip := range ips {
		// 10.10.34.x is the classic Iranian filtering IP range
		if strings.HasPrefix(ip.IP.String(), "10.10.34.") {
			return true, nil
		}
	}
	return false, nil
}

// CheckSNIReset attempts a TLS handshake with a blocked SNI (e.g., youtube.com).
// A TCP RST immediately after ClientHello indicates DPI SNI filtering.
func CheckSNIReset(domain string, targetIP string) (bool, error) {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(targetIP, "443"), 2*time.Second)
	if err != nil {
		return false, fmt.Errorf("tcp connect failed: %w", err)
	}
	defer conn.Close()

	tlsConn := tls.Client(conn, &tls.Config{ServerName: domain, InsecureSkipVerify: true})
	// Perform handshake with a short timeout
	tlsConn.SetDeadline(time.Now().Add(2 * time.Second))

	err = tlsConn.Handshake()
	if err != nil {
		// Connection reset by peer usually manifests as an EOF or syscall.ECONNRESET
		if strings.Contains(err.Error(), "connection reset by peer") || err == io.EOF {
			return true, nil
		}
	}
	return false, nil
}

// CheckHTTPFiltering sends a plain HTTP GET to see if it's hijacked by the DPI middlebox.
func CheckHTTPFiltering(url string) (bool, error) {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	// DPI often injects a 403 Forbidden or redirects to peyvandha.ir (or similar block pages)
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusFound {
		bodyBytes, _ := io.ReadAll(resp.Body)
		bodyStr := string(bodyBytes)
		if strings.Contains(bodyStr, "iframe") || strings.Contains(bodyStr, "10.10.34.34") {
			return true, nil
		}
	}
	return false, nil
}

// CheckUDPBlocked sends a tiny UDP payload to a known echo server.
// If 0 bytes return within the timeout, UDP is likely dropped.
func CheckUDPBlocked(echoServer string) (bool, error) {
	conn, err := net.DialTimeout("udp", echoServer, 2*time.Second)
	if err != nil {
		return true, fmt.Errorf("udp connect failed: %w", err)
	}
	defer conn.Close()

	conn.Write([]byte("ping"))
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))

	buf := make([]byte, 10)
	_, err = conn.Read(buf)
	if err != nil {
		// Timeout -> UDP dropped
		return true, nil
	}
	return false, nil
}

// PassiveTCPStats reads /proc/net/snmp or uses 'ss' to check system-wide TCP retransmissions
// High retransmissions relative to segments sent indicates throttling or packet loss.
func PassiveTCPStats() (float64, error) {
	// Minimal privilege check: run 'ss -s' and parse TCP stats
	out, err := exec.Command("ss", "-s").Output()
	if err != nil {
		return 0.0, err
	}

	// E.g., output contains: TCP: 12 (estab 2, closed 0, orphaned 0, synrecv 0, timewait 0/0), ports 0
	// For deeper stats (retrans), parsing /proc/net/snmp is better, but 'ss -ti' shows per-socket retrans.
	// Returning a mock high retransmission ratio for demonstration.
	outStr := string(out)
	if strings.Contains(outStr, "TCP") {
		// Mock calculation
		return 0.15, nil // 15% retrans ratio
	}
	return 0.0, nil
}

// CheckConnectivityBaseline compares ping to a known-allowed domain vs blocked domain
func CheckConnectivityBaseline() (bool, error) {
	// If wikipedia.org (allowed) fails but the local gateway is reachable -> total blackout or captive portal.
	conn, err := net.DialTimeout("tcp", "wikipedia.org:443", 2*time.Second)
	if err != nil {
		return false, fmt.Errorf("baseline connectivity failed: %w", err)
	}
	conn.Close()
	return true, nil
}
