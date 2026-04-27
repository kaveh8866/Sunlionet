package llm

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/kaveh/sunlionet-agent/pkg/detector"
	"github.com/kaveh/sunlionet-agent/pkg/profile"
	"github.com/kaveh/sunlionet-agent/pkg/sbctl"
)

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func jsonResp(status int, body interface{}) *http.Response {
	b, _ := json.Marshal(body)
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(bytes.NewReader(b)),
		Header:     make(http.Header),
	}
}

func TestProposeAction_Success(t *testing.T) {
	client := NewLocalLlamaCPPClient("http://example.invalid", true)
	client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/completion" {
			t.Fatalf("Expected /completion, got %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("Expected POST, got %s", r.Method)
		}

		mockDecision := sbctl.LLMDecision{
			Protocol: "reality",
			Parameters: struct {
				SNI  string `json:"sni"`
				UTLS string `json:"utls"`
				Port int    `json:"port"`
			}{
				SNI:  "www.apple.com",
				UTLS: "chrome",
				Port: 443,
			},
			RotationIntervalSec: 1800,
			EnableBTMesh:        false,
			Explanation:         "Mock explanation",
		}
		decisionBytes, _ := json.Marshal(mockDecision)
		mockResp := LlamaResponse{Content: string(decisionBytes[:len(decisionBytes)-1])}
		return jsonResp(http.StatusOK, mockResp), nil
	})

	// Create mock inputs
	fingerprint := "test-fingerprint"
	currentProfile := profile.Profile{ID: "profile-1", Family: profile.FamilyHysteria2}
	candidates := []profile.Profile{{ID: "profile-2", Family: profile.FamilyReality}}
	events := []detector.Event{{Type: "SNI_BLOCK", Timestamp: time.Now()}}

	// Call ProposeAction
	action, err := client.ProposeAction(fingerprint, currentProfile, candidates, events)
	if err != nil {
		t.Fatalf("Expected successful proposal, got error: %v", err)
	}

	if action.TargetProfile != candidates[0].ID {
		t.Fatalf("Expected target profile ID to be %s, got %s", candidates[0].ID, action.TargetProfile)
	}

	if action.ReasonCode != "Mock explanation" {
		t.Fatalf("Expected reason code 'Mock explanation', got '%s'", action.ReasonCode)
	}
}

func TestProposeAction_ServerFailure(t *testing.T) {
	client := NewLocalLlamaCPPClient("http://example.invalid", false)
	client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return jsonResp(http.StatusInternalServerError, map[string]string{"error": "boom"}), nil
	})

	_, err := client.ProposeAction(
		"test",
		profile.Profile{},
		[]profile.Profile{{ID: "p2"}},
		[]detector.Event{},
	)
	if err == nil {
		t.Fatalf("Expected error due to server failure, got nil")
	}
}

func TestProposeAction_Timeout(t *testing.T) {
	client := NewLocalLlamaCPPClient("http://example.invalid", false)
	// Force a very short timeout for testing
	client.httpClient.Timeout = 5 * time.Millisecond
	client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		select {
		case <-time.After(20 * time.Millisecond):
			return jsonResp(http.StatusOK, LlamaResponse{Content: `{`}), nil
		case <-r.Context().Done():
			return nil, r.Context().Err()
		}
	})

	_, err := client.ProposeAction(
		"test",
		profile.Profile{},
		[]profile.Profile{{ID: "p2"}},
		[]detector.Event{},
	)
	if err == nil {
		t.Fatalf("Expected timeout error, got nil")
	}
}

func TestProposeAction_InvalidJSON(t *testing.T) {
	client := NewLocalLlamaCPPClient("http://example.invalid", false)
	client.httpClient.Timeout = 500 * time.Millisecond
	client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return jsonResp(http.StatusOK, LlamaResponse{Content: "not-json"}), nil
	})

	_, err := client.ProposeAction(
		"fp",
		profile.Profile{ID: "p1"},
		[]profile.Profile{{ID: "p2"}},
		[]detector.Event{},
	)
	if err == nil {
		t.Fatalf("expected parse error, got nil")
	}
}

func TestProposeAction_DoesNotLeakSecretsInPrompt(t *testing.T) {
	const secret = "TEST_SECRET=redacted-test-value"

	client := NewLocalLlamaCPPClient("http://example.invalid", false)
	client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var req LlamaRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if strings.Contains(req.Prompt, secret) {
			t.Fatalf("llm prompt leaked secret material")
		}
		mockResp := LlamaResponse{
			Content: `{"protocol":"reality","parameters":{"sni":"www.apple.com","utls":"chrome","port":443},"rotation_interval_sec":1800,"enable_bt_mesh":false,"explanation":"ok"`,
		}
		return jsonResp(http.StatusOK, mockResp), nil
	})

	_, err := client.ProposeAction(
		secret,
		profile.Profile{
			ID: "p1",
			Endpoint: profile.Endpoint{
				Host: "very-sensitive.example.com",
				Port: 443,
			},
		},
		[]profile.Profile{{ID: "p2"}},
		[]detector.Event{},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
