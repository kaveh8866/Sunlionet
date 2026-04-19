package assistant

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/kaveh/shadownet-agent/pkg/aipolicy"
)

type Controller struct {
	Local  Provider
	Remote Provider
	Audit  AuditSink
}

func (c *Controller) Invoke(ctx context.Context, pol *aipolicy.Policy, req InvokeRequest) (InvokeResult, error) {
	req.Normalize()
	if err := req.Validate(); err != nil {
		return InvokeResult{}, err
	}
	if pol == nil {
		return InvokeResult{}, errors.New("assistant: policy is nil")
	}

	start := time.Now()
	ev := AuditEvent{
		Scope:     req.Scope,
		Action:    req.Action,
		Backend:   req.Backend,
		Redaction: req.Redaction,
		ItemCount: len(req.Items),
		StartedAt: start,
	}
	defer func() {
		if c != nil && c.Audit != nil {
			_ = c.Audit.Record(ctx, ev)
		}
	}()

	allowed := pol.IsAllowed(time.Now(), req.Scope, req.Action)
	ev.Allowed = allowed
	if !allowed {
		ev.Error = "not_allowed"
		ev.FinishedAt = time.Now()
		return InvokeResult{}, errors.New("assistant: not allowed by policy")
	}

	if req.Backend == BackendRemote && req.Redaction == RedactionOff {
		ev.Error = "remote_requires_redaction"
		ev.FinishedAt = time.Now()
		return InvokeResult{}, errors.New("assistant: remote backend requires redaction")
	}

	prompt, inBytes := BuildPrompt(req.Items, req.Redaction, req.MaxItems, req.MaxBytes)
	ev.InputBytes = inBytes
	if prompt == "" {
		ev.Error = "empty_prompt"
		ev.FinishedAt = time.Now()
		return InvokeResult{}, errors.New("assistant: empty prompt")
	}

	prov, err := c.pickProvider(req.Backend)
	if err != nil {
		ev.Error = err.Error()
		ev.FinishedAt = time.Now()
		return InvokeResult{}, err
	}

	out, err := prov.Complete(ctx, prompt)
	if err != nil {
		ev.Error = err.Error()
		ev.FinishedAt = time.Now()
		return InvokeResult{}, err
	}
	ev.OutputBytes = len(out)
	ev.FinishedAt = time.Now()

	return InvokeResult{
		Text:      out,
		Backend:   req.Backend,
		CreatedAt: time.Now().Unix(),
	}, nil
}

func (c *Controller) pickProvider(backend Backend) (Provider, error) {
	if c == nil {
		return nil, errors.New("assistant: controller is nil")
	}
	switch backend {
	case BackendLocal:
		if c.Local == nil {
			return nil, errors.New("assistant: local provider is nil")
		}
		return c.Local, nil
	case BackendRemote:
		if c.Remote == nil {
			return nil, errors.New("assistant: remote provider is nil")
		}
		return c.Remote, nil
	default:
		return nil, fmt.Errorf("assistant: unknown backend: %q", backend)
	}
}
