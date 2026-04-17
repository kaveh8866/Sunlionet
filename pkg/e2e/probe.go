package e2e

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type FailureReason string

const (
	ReasonOK             FailureReason = "OK"
	ReasonConfigError    FailureReason = "CONFIG_ERROR"
	ReasonBinaryMissing  FailureReason = "BINARY_MISSING"
	ReasonNetworkBlocked FailureReason = "NETWORK_BLOCKED"
	ReasonDNSFailure     FailureReason = "DNS_FAILURE"
	ReasonTimeout        FailureReason = "TIMEOUT"
	ReasonUnknown        FailureReason = "UNKNOWN"
)

type ProbeResult struct {
	Status     string        `json:"status"`
	Reason     FailureReason `json:"reason"`
	TargetURL  string        `json:"target_url"`
	ProxyURL   string        `json:"proxy_url"`
	HTTPStatus int           `json:"http_status,omitempty"`
	DurationMS int64         `json:"duration_ms"`
	Error      string        `json:"error,omitempty"`
	ObservedAt int64         `json:"observed_at_unix"`
}

func HTTPProxyProbe(ctx context.Context, proxyURL string, targetURL string) ProbeResult {
	start := time.Now()
	res := ProbeResult{
		Status:     "failed",
		Reason:     ReasonUnknown,
		TargetURL:  targetURL,
		ProxyURL:   proxyURL,
		ObservedAt: time.Now().Unix(),
	}

	px, err := url.Parse(proxyURL)
	if err != nil {
		res.Error = err.Error()
		res.DurationMS = time.Since(start).Milliseconds()
		return res
	}

	transport := &http.Transport{
		Proxy: http.ProxyURL(px),
		DialContext: (&net.Dialer{
			Timeout:   6 * time.Second,
			KeepAlive: 20 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		TLSHandshakeTimeout:   6 * time.Second,
		ResponseHeaderTimeout: 6 * time.Second,
		ExpectContinueTimeout: 2 * time.Second,
		IdleConnTimeout:       20 * time.Second,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		res.Error = err.Error()
		res.DurationMS = time.Since(start).Milliseconds()
		return res
	}

	resp, err := client.Do(req)
	if err != nil {
		res.Error = err.Error()
		res.Reason = ClassifyError(err, "")
		res.DurationMS = time.Since(start).Milliseconds()
		return res
	}
	defer resp.Body.Close()

	res.HTTPStatus = resp.StatusCode
	_, _ = io.CopyN(io.Discard, resp.Body, 64*1024)

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		res.Error = "unexpected http status: " + resp.Status
		res.Reason = ReasonUnknown
		res.DurationMS = time.Since(start).Milliseconds()
		return res
	}

	res.Status = "ok"
	res.Reason = ReasonOK
	res.DurationMS = time.Since(start).Milliseconds()
	return res
}

func TestConnection(proxyAddr string) error {
	proxyURL := strings.TrimSpace(proxyAddr)
	if proxyURL == "" {
		return fmt.Errorf("missing proxy address")
	}
	if !strings.Contains(proxyURL, "://") {
		proxyURL = "http://" + proxyURL
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	res := HTTPProxyProbe(ctx, proxyURL, "https://example.com")
	if res.Status == "ok" {
		return nil
	}
	if strings.TrimSpace(res.Error) == "" {
		return fmt.Errorf("probe failed: reason=%s", res.Reason)
	}
	return fmt.Errorf("probe failed: reason=%s err=%s", res.Reason, res.Error)
}

func ClassifyError(err error, singBoxLogTail string) FailureReason {
	if err == nil {
		return ReasonOK
	}

	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return ReasonDNSFailure
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return ReasonTimeout
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return ReasonTimeout
	}

	msg := strings.ToLower(err.Error())
	log := strings.ToLower(singBoxLogTail)
	combined := msg + "\n" + log

	if strings.Contains(combined, "sing-box check failed") || strings.Contains(combined, "invalid") && strings.Contains(combined, "config") {
		return ReasonConfigError
	}
	if strings.Contains(combined, "binary not found") || strings.Contains(combined, "no such file") && strings.Contains(combined, "sing-box") {
		return ReasonBinaryMissing
	}
	if strings.Contains(combined, "no such host") || strings.Contains(combined, "dns") && strings.Contains(combined, "failed") {
		return ReasonDNSFailure
	}
	if strings.Contains(combined, "i/o timeout") || strings.Contains(combined, "context deadline exceeded") {
		return ReasonTimeout
	}
	if strings.Contains(combined, "connection refused") ||
		strings.Contains(combined, "network is unreachable") ||
		strings.Contains(combined, "no route to host") ||
		strings.Contains(combined, "failed to connect outbound") ||
		strings.Contains(combined, "handshake") && strings.Contains(combined, "failed") {
		return ReasonNetworkBlocked
	}

	return ReasonUnknown
}
