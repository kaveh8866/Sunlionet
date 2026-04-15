package llm

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kaveh/shadownet-agent/pkg/detector"
	"github.com/kaveh/shadownet-agent/pkg/profile"
	"github.com/kaveh/shadownet-agent/pkg/sbctl"
)

func TestProposeAction_Success(t *testing.T) {
	// Create a mock llama.cpp server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request path
		if r.URL.Path != "/completion" {
			t.Fatalf("Expected /completion, got %s", r.URL.Path)
		}

		// Verify request method
		if r.Method != http.MethodPost {
			t.Fatalf("Expected POST, got %s", r.Method)
		}

		// Mock a successful JSON response matching the LLMDecision schema
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

		// The grammar stops generation at `}`, so we simulate llama.cpp leaving it out or returning it
		// The client appends `}` manually because it stopped there.
		mockResp := LlamaResponse{
			Content: string(decisionBytes[:len(decisionBytes)-1]), // Remove the last `}`
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(mockResp)
	}))
	defer mockServer.Close()

	// Initialize the client pointing to our mock server
	client := NewLocalLlamaCPPClient(mockServer.URL, true)

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
	// Create a mock llama.cpp server that returns 500 Internal Server Error
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer mockServer.Close()

	client := NewLocalLlamaCPPClient(mockServer.URL, false)

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
	// Create a mock llama.cpp server that sleeps to simulate timeout
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(20 * time.Millisecond) // Simulate slow response
	}))
	defer mockServer.Close()

	client := NewLocalLlamaCPPClient(mockServer.URL, false)
	// Force a very short timeout for testing
	client.httpClient.Timeout = 5 * time.Millisecond

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
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(LlamaResponse{Content: "not-json"})
	}))
	defer mockServer.Close()

	client := NewLocalLlamaCPPClient(mockServer.URL, false)
	client.httpClient.Timeout = 500 * time.Millisecond

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
	const secret = "SHADOWNET_MASTER_KEY=super-secret-material"

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(mockResp)
	}))
	defer mockServer.Close()

	client := NewLocalLlamaCPPClient(mockServer.URL, false)

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
