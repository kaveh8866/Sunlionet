package assistant

import (
	"errors"
	"fmt"
	"time"

	"github.com/kaveh/sunlionet-agent/pkg/aipolicy"
)

type Backend string

const (
	BackendLocal  Backend = "local"
	BackendRemote Backend = "remote"
)

type RedactionProfile string

const (
	RedactionStrict   RedactionProfile = "strict"
	RedactionModerate RedactionProfile = "moderate"
	RedactionOff      RedactionProfile = "off"
)

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
)

type Item struct {
	Role Role   `json:"role"`
	Text string `json:"text"`
}

func (it Item) Validate() error {
	switch it.Role {
	case RoleUser, RoleAssistant, RoleSystem:
	default:
		return fmt.Errorf("assistant: invalid role: %q", it.Role)
	}
	if it.Text == "" {
		return errors.New("assistant: item text required")
	}
	return nil
}

type InvokeRequest struct {
	Scope       aipolicy.Scope   `json:"scope"`
	Action      aipolicy.Action  `json:"action"`
	Backend     Backend          `json:"backend"`
	Redaction   RedactionProfile `json:"redaction"`
	UserGesture bool             `json:"user_gesture"`

	Items []Item `json:"items"`

	MaxItems int `json:"max_items,omitempty"`
	MaxBytes int `json:"max_bytes,omitempty"`
}

func (r *InvokeRequest) Normalize() {
	if r.MaxItems <= 0 {
		r.MaxItems = 24
	}
	if r.MaxBytes <= 0 {
		r.MaxBytes = 24 * 1024
	}
	if r.Redaction == "" {
		r.Redaction = RedactionStrict
	}
	if r.Backend == "" {
		r.Backend = BackendLocal
	}
}

func (r *InvokeRequest) Validate() error {
	if r == nil {
		return errors.New("assistant: request is nil")
	}
	if r.Scope.ID == "" {
		return errors.New("assistant: scope id required")
	}
	if r.Action == "" {
		return errors.New("assistant: action required")
	}
	switch r.Backend {
	case BackendLocal, BackendRemote:
	default:
		return fmt.Errorf("assistant: invalid backend: %q", r.Backend)
	}
	switch r.Redaction {
	case RedactionStrict, RedactionModerate, RedactionOff:
	default:
		return fmt.Errorf("assistant: invalid redaction profile: %q", r.Redaction)
	}
	if !r.UserGesture {
		return errors.New("assistant: user_gesture required")
	}
	if len(r.Items) == 0 {
		return errors.New("assistant: items required")
	}
	for i := range r.Items {
		if err := r.Items[i].Validate(); err != nil {
			return err
		}
	}
	return nil
}

type InvokeResult struct {
	Text      string  `json:"text"`
	Backend   Backend `json:"backend"`
	CreatedAt int64   `json:"created_at"`
}

type AuditEvent struct {
	Scope     aipolicy.Scope   `json:"scope"`
	Action    aipolicy.Action  `json:"action"`
	Backend   Backend          `json:"backend"`
	Redaction RedactionProfile `json:"redaction"`

	ItemCount   int `json:"item_count"`
	InputBytes  int `json:"input_bytes"`
	OutputBytes int `json:"output_bytes"`

	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
	Allowed    bool      `json:"allowed"`
	Error      string    `json:"error,omitempty"`
}
