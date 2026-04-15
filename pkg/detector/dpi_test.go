package detector

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
	"time"
)

type fakeResolver struct {
	ips []net.IPAddr
	err error
}

func (r *fakeResolver) LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error) {
	return r.ips, r.err
}

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

func TestCheckDNSPoisoning_DetectsKnownRange(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	poisoned, err := CheckDNSPoisoningWith(ctx, &fakeResolver{
		ips: []net.IPAddr{{IP: net.ParseIP("10.10.34.34")}},
	}, "twitter.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !poisoned {
		t.Fatalf("expected poisoning to be detected")
	}
}

func TestCheckDNSPoisoning_NotPoisoned(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	poisoned, err := CheckDNSPoisoningWith(ctx, &fakeResolver{
		ips: []net.IPAddr{{IP: net.ParseIP("8.8.8.8")}},
	}, "example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if poisoned {
		t.Fatalf("expected not poisoned")
	}
}

func selfSignedTLSConfig(t *testing.T) *tls.Config {
	t.Helper()

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa key: %v", err)
	}

	serial, err := rand.Int(rand.Reader, big.NewInt(1<<62))
	if err != nil {
		t.Fatalf("serial: %v", err)
	}

	tpl := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName: "localhost",
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost"},
	}

	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("keypair: %v", err)
	}

	return &tls.Config{Certificates: []tls.Certificate{cert}}
}

func TestCheckSNIReset_DetectsResetLikeClose(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	go func() {
		c, err := ln.Accept()
		if err == nil {
			if tc, ok := c.(*net.TCPConn); ok {
				_ = tc.SetLinger(0)
			}
			_ = c.Close()
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	blocked, err := CheckSNIResetWith(ctx, &net.Dialer{}, "blocked.example", ln.Addr().String())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !blocked {
		t.Fatalf("expected blocked due to early close/EOF during handshake")
	}
}

func TestCheckSNIReset_AllowsHandshake(t *testing.T) {
	cfg := selfSignedTLSConfig(t)
	ln, err := tls.Listen("tcp", "127.0.0.1:0", cfg)
	if err != nil {
		t.Fatalf("tls listen: %v", err)
	}
	defer ln.Close()

	go func() {
		c, err := ln.Accept()
		if err == nil {
			if tc, ok := c.(*tls.Conn); ok {
				_ = tc.Handshake()
			}
			time.Sleep(20 * time.Millisecond)
			_ = c.Close()
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	blocked, err := CheckSNIResetWith(ctx, &net.Dialer{}, "localhost", ln.Addr().String())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if blocked {
		t.Fatalf("expected not blocked when TLS handshake succeeds")
	}
}

func TestCheckHTTPFiltering_DetectsInjected403(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`<html><body><iframe src="http://10.10.34.34/"></iframe></body></html>`))
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	blocked, err := CheckHTTPFilteringWith(ctx, srv.Client(), srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !blocked {
		t.Fatalf("expected filtering detection")
	}
}

func TestCheckHTTPFiltering_NotFiltered(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`ok`))
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	blocked, err := CheckHTTPFilteringWith(ctx, srv.Client(), srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if blocked {
		t.Fatalf("expected not filtered")
	}
}

func TestCheckUDPBlocked_AllowsEcho(t *testing.T) {
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("udp listen: %v", err)
	}
	defer pc.Close()

	go func() {
		buf := make([]byte, 32)
		n, addr, err := pc.ReadFrom(buf)
		if err == nil && n > 0 {
			_, _ = pc.WriteTo(buf[:n], addr)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	blocked, err := CheckUDPBlockedWith(ctx, &net.Dialer{}, pc.LocalAddr().String())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if blocked {
		t.Fatalf("expected not blocked when echo replies")
	}
}

func TestCheckUDPBlocked_DetectsDrop(t *testing.T) {
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("udp listen: %v", err)
	}
	defer pc.Close()

	go func() {
		buf := make([]byte, 32)
		_, _, _ = pc.ReadFrom(buf)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	blocked, err := CheckUDPBlockedWith(ctx, &net.Dialer{}, pc.LocalAddr().String())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !blocked {
		t.Fatalf("expected blocked when no reply arrives")
	}
}

func TestCheckConnectivityBaseline_LocalTCP(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	go func() {
		c, err := ln.Accept()
		if err == nil {
			_ = c.Close()
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ok, err := CheckConnectivityBaselineWith(ctx, &net.Dialer{}, ln.Addr().String())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatalf("expected baseline ok")
	}
}
