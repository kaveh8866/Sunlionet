package aipolicy

import (
	"errors"
	"fmt"
	"time"
)

type ScopeType string

const (
	ScopeChat     ScopeType = "chat"
	ScopeGroup    ScopeType = "group"
	ScopeDocument ScopeType = "document"
)

type Action string

const (
	ActionSummarize Action = "summarize"
	ActionTranslate Action = "translate"
	ActionDraft     Action = "draft"
	ActionRetrieve  Action = "retrieve"
	ActionModerate  Action = "moderate"
)

type Scope struct {
	Type ScopeType `json:"type"`
	ID   string    `json:"id"`
}

type Grant struct {
	Scope     Scope    `json:"scope"`
	Actions   []Action `json:"actions"`
	CreatedAt int64    `json:"created_at"`
	ExpiresAt int64    `json:"expires_at"`
	MaxItems  int      `json:"max_items,omitempty"`
	Purpose   string   `json:"purpose,omitempty"`
}

type Policy struct {
	Grants []Grant `json:"grants"`
}

func (p *Policy) Validate() error {
	if p == nil {
		return errors.New("aipolicy: policy is nil")
	}
	for i := range p.Grants {
		g := p.Grants[i]
		if err := g.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (g *Grant) Validate() error {
	if g == nil {
		return errors.New("aipolicy: grant is nil")
	}
	if g.Scope.ID == "" {
		return errors.New("aipolicy: scope id required")
	}
	switch g.Scope.Type {
	case ScopeChat, ScopeGroup, ScopeDocument:
	default:
		return fmt.Errorf("aipolicy: invalid scope type: %q", g.Scope.Type)
	}
	if len(g.Actions) == 0 {
		return errors.New("aipolicy: actions required")
	}
	if g.CreatedAt <= 0 || g.ExpiresAt <= 0 {
		return errors.New("aipolicy: created_at/expires_at required")
	}
	if g.ExpiresAt < g.CreatedAt {
		return errors.New("aipolicy: expires_at before created_at")
	}
	if g.MaxItems < 0 {
		return errors.New("aipolicy: max_items must be >= 0")
	}
	return nil
}

func NewGrant(scope Scope, actions []Action, ttl time.Duration) Grant {
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	now := time.Now()
	return Grant{
		Scope:     scope,
		Actions:   append([]Action(nil), actions...),
		CreatedAt: now.Unix(),
		ExpiresAt: now.Add(ttl).Unix(),
	}
}

func (p *Policy) IsAllowed(now time.Time, scope Scope, action Action) bool {
	if p == nil {
		return false
	}
	for i := range p.Grants {
		g := p.Grants[i]
		if g.Scope.Type != scope.Type || g.Scope.ID != scope.ID {
			continue
		}
		if now.Unix() > g.ExpiresAt {
			continue
		}
		for j := range g.Actions {
			if g.Actions[j] == action {
				return true
			}
		}
	}
	return false
}
