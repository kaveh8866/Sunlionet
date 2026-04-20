package relay

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type HTTPClient struct {
	baseURL string
	http    *http.Client
}

func NewHTTPClient(baseURL string) *HTTPClient {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	return &HTTPClient{
		baseURL: baseURL,
		http: &http.Client{
			Timeout: 12 * time.Second,
		},
	}
}

func (c *HTTPClient) Push(ctx context.Context, req PushRequest) (MessageID, error) {
	if req.PoWBits > 0 && strings.TrimSpace(req.PoWNonceB64URL) == "" {
		nonce, err := findPoWNonce(req.Mailbox, req.Envelope, req.PoWBits, 250_000)
		if err != nil {
			return "", err
		}
		req.PoWNonceB64URL = nonce
	}
	if err := req.Validate(); err != nil {
		return "", err
	}
	var resp struct {
		ID string `json:"id"`
	}
	if err := c.postJSON(ctx, "/v1/push", req, &resp); err != nil {
		return "", err
	}
	if strings.TrimSpace(resp.ID) == "" {
		return "", errors.New("relay: missing id in response")
	}
	return MessageID(resp.ID), nil
}

func (c *HTTPClient) Pull(ctx context.Context, req PullRequest) ([]Message, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	req = req.Normalize()
	var resp []Message
	if err := c.postJSON(ctx, "/v1/pull", req, &resp); err != nil {
		return nil, err
	}
	for i := range resp {
		if err := resp[i].Validate(); err != nil {
			return nil, err
		}
	}
	return resp, nil
}

func (c *HTTPClient) Ack(ctx context.Context, req AckRequest) error {
	if err := req.Validate(); err != nil {
		return err
	}
	var resp struct {
		Status string `json:"status"`
	}
	return c.postJSON(ctx, "/v1/ack", req, &resp)
}

func (c *HTTPClient) postJSON(ctx context.Context, path string, req any, out any) error {
	if c == nil || c.http == nil {
		return errors.New("relay: http client is nil")
	}
	if strings.TrimSpace(c.baseURL) == "" {
		return errors.New("relay: baseURL is empty")
	}
	body, err := json.Marshal(req)
	if err != nil {
		return err
	}
	u := c.baseURL + path
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return err
	}
	r.Header.Set("Content-Type", "application/json")
	res, err := c.http.Do(r)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	b, _ := io.ReadAll(res.Body)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		var errObj struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(b, &errObj) == nil && strings.TrimSpace(errObj.Error) != "" {
			return fmt.Errorf("relay: http %d: %s", res.StatusCode, errObj.Error)
		}
		return fmt.Errorf("relay: http %d", res.StatusCode)
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(b, out)
}

func findPoWNonce(mailbox MailboxID, envelope Envelope, bits int, maxIter int) (string, error) {
	if bits <= 0 {
		return "", nil
	}
	if maxIter <= 0 {
		maxIter = 1
	}
	var salt [8]byte
	if _, err := rand.Read(salt[:]); err != nil {
		return "", err
	}
	var nonce [16]byte
	copy(nonce[:8], salt[:])
	for i := 0; i < maxIter; i++ {
		binary.LittleEndian.PutUint64(nonce[8:], uint64(i))
		h := sha256.New()
		_, _ = h.Write([]byte(mailbox))
		_, _ = h.Write([]byte{0})
		_, _ = h.Write([]byte(envelope))
		_, _ = h.Write([]byte{0})
		_, _ = h.Write(nonce[:])
		sum := h.Sum(nil)
		if hashHasLeadingZeroBits(sum, bits) {
			return base64.RawURLEncoding.EncodeToString(nonce[:]), nil
		}
	}
	return "", errors.New("relay: proof of work failed")
}
