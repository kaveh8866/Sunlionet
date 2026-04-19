package assistant

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type LlamaCPPProvider struct {
	ServerURL   string
	NPredict    int
	Temperature float64
	Timeout     time.Duration
	StopOnEOT   bool
	httpClient  *http.Client
}

type llamaCPPRequest struct {
	Prompt      string   `json:"prompt"`
	NPredict    int      `json:"n_predict"`
	Temperature float64  `json:"temperature"`
	Stop        []string `json:"stop,omitempty"`
}

type llamaCPPResponse struct {
	Content string `json:"content"`
}

func (p *LlamaCPPProvider) Complete(ctx context.Context, prompt string) (string, error) {
	if p == nil {
		return "", errors.New("assistant: provider is nil")
	}
	serverURL := strings.TrimRight(strings.TrimSpace(p.ServerURL), "/")
	if serverURL == "" {
		return "", errors.New("assistant: llama.cpp server_url is empty")
	}
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return "", errors.New("assistant: empty prompt")
	}

	nPredict := p.NPredict
	if nPredict <= 0 {
		nPredict = 256
	}
	temp := p.Temperature
	if temp <= 0 {
		temp = 0.2
	}
	timeout := p.Timeout
	if timeout <= 0 {
		timeout = 12 * time.Second
	}
	if p.httpClient == nil {
		p.httpClient = &http.Client{Timeout: timeout}
	}

	reqPayload := llamaCPPRequest{
		Prompt:      prompt,
		NPredict:    nPredict,
		Temperature: temp,
	}
	if p.StopOnEOT {
		reqPayload.Stop = []string{"</s>", "<|eot_id|>"}
	}
	body, err := json.Marshal(reqPayload)
	if err != nil {
		return "", err
	}

	u := serverURL + "/completion"
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	r.Header.Set("Content-Type", "application/json")

	res, err := p.httpClient.Do(r)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	b, _ := io.ReadAll(res.Body)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return "", fmt.Errorf("assistant: llama.cpp http %d", res.StatusCode)
	}
	var out llamaCPPResponse
	if err := json.Unmarshal(b, &out); err != nil {
		return "", err
	}
	text := strings.TrimSpace(out.Content)
	if text == "" {
		return "", errors.New("assistant: empty llama.cpp response")
	}
	return text, nil
}
