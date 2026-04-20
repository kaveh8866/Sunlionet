package detector

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type Resolver interface {
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
}

type Dialer interface {
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
}

type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// CheckDNSPoisoning resolves a domain known to be blocked in Iran (e.g., twitter.com).
// If the response is a private IP or the known 10.10.34.x block page IP, it's poisoned.
func CheckDNSPoisoning(domain string) (bool, error) {
	// 1s timeout to ensure under 5s total budget
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	return CheckDNSPoisoningWith(ctx, net.DefaultResolver, domain)
}

func CheckDNSPoisoningWith(ctx context.Context, resolver Resolver, domain string) (bool, error) {
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
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return CheckSNIResetWith(ctx, &net.Dialer{}, domain, net.JoinHostPort(targetIP, "443"))
}

func CheckSNIResetWith(ctx context.Context, dialer Dialer, domain string, targetAddr string) (bool, error) {
	conn, err := dialer.DialContext(ctx, "tcp", targetAddr)
	if err != nil {
		if isConnResetLike(err) {
			return true, nil
		}
		return false, fmt.Errorf("tcp connect failed: %w", err)
	}
	defer conn.Close()

	tlsConn := tls.Client(conn, &tls.Config{ServerName: domain})
	if deadline, ok := ctx.Deadline(); ok {
		_ = tlsConn.SetDeadline(deadline)
	} else {
		_ = tlsConn.SetDeadline(time.Now().Add(2 * time.Second))
	}

	err = tlsConn.Handshake()
	if err != nil && isConnResetLike(err) {
		return true, nil
	}
	return false, nil
}

func isConnResetLike(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) {
		return true
	}
	if errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	if errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.ECONNABORTED) || errors.Is(err, syscall.EPIPE) {
		return true
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		if errors.Is(opErr.Err, syscall.ECONNRESET) || errors.Is(opErr.Err, syscall.ECONNABORTED) || errors.Is(opErr.Err, syscall.EPIPE) {
			return true
		}
		if errors.Is(opErr.Err, io.EOF) {
			return true
		}
		if errors.Is(opErr.Err, io.ErrUnexpectedEOF) {
			return true
		}
	}

	errStr := strings.ToLower(err.Error())
	if strings.Contains(errStr, "connection reset") || strings.Contains(errStr, "broken pipe") || strings.Contains(errStr, "forcibly closed") {
		return true
	}
	if strings.Contains(errStr, "unexpected eof") || strings.HasSuffix(errStr, "eof") || strings.Contains(errStr, ": eof") {
		return true
	}
	return false
}

// CheckHTTPFiltering sends a plain HTTP GET to see if it's hijacked by the DPI middlebox.
func CheckHTTPFiltering(url string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return CheckHTTPFilteringWith(ctx, &http.Client{Timeout: 2 * time.Second}, url)
}

func CheckHTTPFilteringWith(ctx context.Context, client HTTPDoer, url string) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, err
	}

	resp, err := client.Do(req)
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
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return CheckUDPBlockedWith(ctx, &net.Dialer{}, echoServer)
}

func CheckUDPBlockedWith(ctx context.Context, dialer Dialer, echoServer string) (bool, error) {
	conn, err := dialer.DialContext(ctx, "udp", echoServer)
	if err != nil {
		return true, fmt.Errorf("udp connect failed: %w", err)
	}
	defer conn.Close()

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	} else {
		_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	}

	conn.Write([]byte("ping"))

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
	b, err := os.ReadFile("/proc/net/snmp")
	if err != nil {
		return 0, err
	}

	lines := strings.Split(string(b), "\n")
	for i := 0; i+1 < len(lines); i++ {
		if !strings.HasPrefix(lines[i], "Tcp:") || !strings.HasPrefix(lines[i+1], "Tcp:") {
			continue
		}
		header := strings.Fields(lines[i])
		values := strings.Fields(lines[i+1])
		if len(header) < 2 || len(values) < 2 || len(header) != len(values) {
			continue
		}

		var retransSegs uint64
		var outSegs uint64
		var haveRetrans bool
		var haveOut bool
		for j := 1; j < len(header); j++ {
			v, err := strconv.ParseUint(values[j], 10, 64)
			if err != nil {
				continue
			}
			switch header[j] {
			case "RetransSegs":
				retransSegs = v
				haveRetrans = true
			case "OutSegs":
				outSegs = v
				haveOut = true
			}
		}

		if !haveRetrans || !haveOut {
			return 0, fmt.Errorf("tcp stats unavailable")
		}
		if outSegs == 0 {
			return 0, fmt.Errorf("tcp stats unavailable")
		}
		return float64(retransSegs) / float64(outSegs), nil
	}

	return 0, fmt.Errorf("tcp stats unavailable")
}

// CheckConnectivityBaseline compares ping to a known-allowed domain vs blocked domain
func CheckConnectivityBaseline() (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return CheckConnectivityBaselineWith(ctx, &net.Dialer{}, "wikipedia.org:443")
}

func CheckConnectivityBaselineWith(ctx context.Context, dialer Dialer, address string) (bool, error) {
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return false, fmt.Errorf("baseline connectivity failed: %w", err)
	}
	conn.Close()
	return true, nil
}
