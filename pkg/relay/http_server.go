package relay

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const maxRelayRequestBytes = 512 * 1024

type Server struct {
	relay Relay
	srv   *http.Server
	addr  string
}

type ServerOptions struct {
	AllowNonLocal          bool
	MinPoWBits             int
	IPRateLimitPerMin      int
	MailboxRateLimitPerMin int
	AuthToken              string
}

func (o ServerOptions) normalize() ServerOptions {
	out := o
	if out.MinPoWBits < 0 {
		out.MinPoWBits = 0
	}
	if out.MinPoWBits > 28 {
		out.MinPoWBits = 28
	}
	if out.IPRateLimitPerMin < 0 {
		out.IPRateLimitPerMin = 0
	}
	if out.MailboxRateLimitPerMin < 0 {
		out.MailboxRateLimitPerMin = 0
	}
	if out.IPRateLimitPerMin == 0 {
		out.IPRateLimitPerMin = 1200
	}
	if out.MailboxRateLimitPerMin == 0 {
		out.MailboxRateLimitPerMin = 240
	}
	if out.IPRateLimitPerMin > 12000 {
		out.IPRateLimitPerMin = 12000
	}
	if out.MailboxRateLimitPerMin > 6000 {
		out.MailboxRateLimitPerMin = 6000
	}
	out.AuthToken = strings.TrimSpace(out.AuthToken)
	return out
}

func NewLocalServer(addr string, relay Relay) (*Server, error) {
	return NewServer(addr, relay, ServerOptions{AllowNonLocal: false})
}

func NewServer(addr string, relay Relay, opts ServerOptions) (*Server, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return nil, fmt.Errorf("relay: missing addr")
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("relay: invalid addr %q (expected host:port): %w", addr, err)
	}
	host = strings.TrimSpace(host)
	if !opts.AllowNonLocal {
		if host != "127.0.0.1" && host != "localhost" {
			return nil, fmt.Errorf("relay: server must bind to localhost only, got host=%q", host)
		}
	}
	if relay == nil {
		return nil, fmt.Errorf("relay: relay is nil")
	}
	opts = opts.normalize()

	mux := http.NewServeMux()
	applySecurityHeaders := func(w http.ResponseWriter) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		w.Header().Set("Content-Security-Policy", "frame-ancestors 'none'; base-uri 'self'; object-src 'none'")
	}
	writeJSON := func(w http.ResponseWriter, status int, v any) {
		applySecurityHeaders(w)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(v)
	}
	readJSON := func(w http.ResponseWriter, r *http.Request, v any) error {
		ct := strings.TrimSpace(r.Header.Get("Content-Type"))
		if ct != "" && !strings.HasPrefix(ct, "application/json") {
			return fmt.Errorf("unsupported content-type")
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxRelayRequestBytes)
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(v); err != nil {
			return err
		}
		var extra any
		if err := dec.Decode(&extra); err == nil {
			return fmt.Errorf("invalid json")
		}
		return nil
	}

	lim := newTokenLimiter()
	requireAuth := func(w http.ResponseWriter, r *http.Request) bool {
		if opts.AuthToken == "" {
			return true
		}
		authz := strings.TrimSpace(r.Header.Get("Authorization"))
		const bearerPrefix = "Bearer "
		if !strings.HasPrefix(authz, bearerPrefix) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return false
		}
		token := strings.TrimSpace(strings.TrimPrefix(authz, bearerPrefix))
		if subtle.ConstantTimeCompare([]byte(token), []byte(opts.AuthToken)) != 1 {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return false
		}
		return true
	}
	allowRequest := func(r *http.Request, mailbox MailboxID) error {
		ip := remoteIP(r)
		if ip == "" {
			return errors.New("relay: missing remote ip")
		}
		now := time.Now()
		if opts.IPRateLimitPerMin > 0 {
			rate := float64(opts.IPRateLimitPerMin) / 60.0
			if !lim.Allow("ip:"+ip, rate, float64(opts.IPRateLimitPerMin)/10.0+5, now) {
				return errors.New("relay: rate limited")
			}
		}
		if opts.MailboxRateLimitPerMin > 0 && mailbox != "" {
			rate := float64(opts.MailboxRateLimitPerMin) / 60.0
			if !lim.Allow("mb:"+string(mailbox), rate, float64(opts.MailboxRateLimitPerMin)/10.0+2, now) {
				return errors.New("relay: mailbox rate limited")
			}
		}
		return nil
	}

	mux.HandleFunc("/v1/push", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		if !requireAuth(w, r) {
			return
		}
		var req PushRequest
		if err := readJSON(w, r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if err := allowRequest(r, req.Mailbox); err != nil {
			writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": err.Error()})
			return
		}
		powBits := req.PoWBits
		if opts.MinPoWBits > powBits {
			powBits = opts.MinPoWBits
		}
		if err := VerifyPoW(req.Mailbox, req.Envelope, req.PoWNonceB64URL, powBits); err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
			return
		}
		id, err := relay.Push(r.Context(), req)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"id": string(id)})
	})

	mux.HandleFunc("/v1/pull", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		if !requireAuth(w, r) {
			return
		}
		var req PullRequest
		if err := readJSON(w, r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if err := allowRequest(r, req.Mailbox); err != nil {
			writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": err.Error()})
			return
		}
		msgs, err := relay.Pull(r.Context(), req)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, msgs)
	})

	mux.HandleFunc("/v1/ack", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		if !requireAuth(w, r) {
			return
		}
		var req AckRequest
		if err := readJSON(w, r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if err := allowRequest(r, req.Mailbox); err != nil {
			writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": err.Error()})
			return
		}
		if err := relay.Ack(r.Context(), req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	s := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}
	return &Server{relay: relay, srv: s, addr: addr}, nil
}

func (s *Server) Start() error {
	if s == nil || s.srv == nil {
		return fmt.Errorf("relay: server is nil")
	}
	ln, err := net.Listen("tcp", s.srv.Addr)
	if err != nil {
		return err
	}
	s.addr = ln.Addr().String()
	go func() { _ = s.srv.Serve(ln) }()
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s == nil || s.srv == nil {
		return nil
	}
	return s.srv.Shutdown(ctx)
}

func (s *Server) Addr() string {
	if s == nil {
		return ""
	}
	return strings.TrimSpace(s.addr)
}

type tokenLimiter struct {
	mu      sync.Mutex
	buckets map[string]*tokenBucket
}

type tokenBucket struct {
	tokens float64
	last   time.Time
}

func newTokenLimiter() *tokenLimiter {
	return &tokenLimiter{buckets: make(map[string]*tokenBucket)}
}

func (l *tokenLimiter) Allow(key string, ratePerSec float64, burst float64, now time.Time) bool {
	if ratePerSec <= 0 {
		return true
	}
	if burst < 1 {
		burst = 1
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	b := l.buckets[key]
	if b == nil {
		b = &tokenBucket{tokens: burst, last: now}
		l.buckets[key] = b
	}
	elapsed := now.Sub(b.last).Seconds()
	if elapsed > 0 {
		b.tokens += elapsed * ratePerSec
		if b.tokens > burst {
			b.tokens = burst
		}
		b.last = now
	}
	if b.tokens < 1 {
		return false
	}
	b.tokens -= 1
	return true
}

func remoteIP(r *http.Request) string {
	if r == nil {
		return ""
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(host)
}
