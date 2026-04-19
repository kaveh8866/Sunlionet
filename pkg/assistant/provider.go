package assistant

import (
	"context"
	"errors"
)

type Provider interface {
	Complete(ctx context.Context, prompt string) (string, error)
}

type AuditSink interface {
	Record(ctx context.Context, ev AuditEvent) error
}

type NoopAuditSink struct{}

func (NoopAuditSink) Record(ctx context.Context, ev AuditEvent) error {
	_ = ctx
	_ = ev
	return nil
}

type StaticProvider struct {
	Text string
	Err  error
}

func (p StaticProvider) Complete(ctx context.Context, prompt string) (string, error) {
	_ = ctx
	_ = prompt
	if p.Err != nil {
		return "", p.Err
	}
	if p.Text == "" {
		return "", errors.New("assistant: empty provider response")
	}
	return p.Text, nil
}
