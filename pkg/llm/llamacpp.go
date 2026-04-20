package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"text/template"
	"time"

	"github.com/kaveh/sunlionet-agent/pkg/detector"
	"github.com/kaveh/sunlionet-agent/pkg/policy"
	"github.com/kaveh/sunlionet-agent/pkg/profile"
	"github.com/kaveh/sunlionet-agent/pkg/sbctl"
)

// LocalLlamaCPPClient implements the Advisor interface via a local llama.cpp server
type LocalLlamaCPPClient struct {
	serverURL  string
	httpClient *http.Client
	debugMode  bool
}

func NewLocalLlamaCPPClient(serverURL string, debug bool) *LocalLlamaCPPClient {
	return &LocalLlamaCPPClient{
		serverURL: serverURL, // e.g., http://127.0.0.1:8080
		httpClient: &http.Client{
			Timeout: 10 * time.Second, // Hard timeout to ensure <3s average decisions
		},
		debugMode: debug,
	}
}

// LlamaRequest represents the payload expected by llama.cpp /completion endpoint
type LlamaRequest struct {
	Prompt      string   `json:"prompt"`
	NPredict    int      `json:"n_predict"`
	Temperature float64  `json:"temperature"`
	Stop        []string `json:"stop"`
	Grammar     string   `json:"grammar,omitempty"` // Enforces JSON structure
}

// LlamaResponse represents the response from llama.cpp
type LlamaResponse struct {
	Content string `json:"content"`
}

func (c *LocalLlamaCPPClient) ProposeAction(
	fingerprint string,
	currentProfile profile.Profile,
	candidates []profile.Profile,
	recentEvents []detector.Event,
) (policy.Action, error) {

	// 1. Generate the system prompt (using the logic we built previously)
	prompt, err := c.buildPrompt(fingerprint, currentProfile, candidates, recentEvents)
	if err != nil {
		return policy.Action{}, err
	}

	// GBBNF Grammar to force strictly valid JSON matching the LLMDecision schema
	// This prevents the model from hallucinating markdown backticks or conversational text
	jsonGrammar := `
root ::= "{" ws "\"protocol\"" ws ":" ws string "," ws "\"parameters\"" ws ":" ws parameters "," ws "\"rotation_interval_sec\"" ws ":" ws number "," ws "\"enable_bt_mesh\"" ws ":" ws boolean "," ws "\"explanation\"" ws ":" ws string "}"
parameters ::= "{" ws "\"sni\"" ws ":" ws string "," ws "\"utls\"" ws ":" ws string "," ws "\"port\"" ws ":" ws number "}"
string ::= "\"" [^"]* "\""
number ::= [0-9]+
boolean ::= "true" | "false"
ws ::= [ \t\n]*
`

	reqPayload := LlamaRequest{
		Prompt:      prompt,
		NPredict:    256,           // Keep output short
		Temperature: 0.1,           // Highly deterministic
		Stop:        []string{"}"}, // Stop generation immediately after JSON closes
		Grammar:     jsonGrammar,
	}

	body, _ := json.Marshal(reqPayload)

	if c.debugMode {
		log.Printf("[LLM-DEBUG] Sending prompt to llama.cpp (%d bytes)", len(prompt))
	}

	// 2. Invoke local llama.cpp server
	req, err := http.NewRequestWithContext(
		context.Background(),
		"POST",
		fmt.Sprintf("%s/completion", c.serverURL),
		bytes.NewReader(body),
	)
	if err != nil {
		return policy.Action{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return policy.Action{}, fmt.Errorf("llama.cpp unreachable or timed out: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return policy.Action{}, fmt.Errorf("llama.cpp returned HTTP %d", resp.StatusCode)
	}

	respBody, _ := io.ReadAll(resp.Body)
	var llamaResp LlamaResponse
	if err := json.Unmarshal(respBody, &llamaResp); err != nil {
		return policy.Action{}, err
	}

	// Since we stopped on "}", we need to append it back for valid JSON parsing
	rawOutput := llamaResp.Content + "}"

	if c.debugMode {
		log.Printf("[LLM-DEBUG] Decision received in %v:\n%s", time.Since(start), rawOutput)
	}

	// 3. Parse into sbctl.LLMDecision
	var decision sbctl.LLMDecision
	if err := json.Unmarshal([]byte(rawOutput), &decision); err != nil {
		return policy.Action{}, fmt.Errorf("failed to parse LLM JSON: %w", err)
	}

	// Convert LLMDecision to generic policy.Action (mock mapping for interface compatibility)
	return policy.Action{
		TargetProfile: candidates[0].ID, // In reality, match protocol/SNI back to a candidate ID
		ReasonCode:    decision.Explanation,
		Confidence:    0.9,
	}, nil
}

// buildPrompt wraps the template execution from our previous implementation
func (c *LocalLlamaCPPClient) buildPrompt(
	fingerprint string,
	currentProfile profile.Profile,
	candidates []profile.Profile,
	recentEvents []detector.Event,
) (string, error) {
	tmpl, err := template.New("prompt").Parse(sysPrompt)
	if err != nil {
		return "", err
	}

	data := struct {
		Fingerprint    string
		CurrentProfile profile.Profile
		Candidates     []profile.Profile
		RecentEvents   []detector.Event
		CPU            int
		RAM            int
		Battery        int
	}{
		Fingerprint:    "redacted",
		CurrentProfile: currentProfile,
		Candidates:     candidates,
		RecentEvents:   recentEvents,
		CPU:            0,
		RAM:            0,
		Battery:        0,
	}

	var promptBuf bytes.Buffer
	if err := tmpl.Execute(&promptBuf, data); err != nil {
		return "", err
	}

	_ = fingerprint
	return promptBuf.String(), nil
}
