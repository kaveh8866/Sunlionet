package mesh_controller

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/kaveh/sunlionet-agent/core/mesh/security"
	"github.com/kaveh/sunlionet-agent/core/proximity/identity_manager"
	"github.com/kaveh/sunlionet-agent/core/proximity/message_router"
	"github.com/kaveh/sunlionet-agent/core/transport/transport_manager"
	"github.com/kaveh/sunlionet-agent/core/transport/transport_selector"
)

type Controller struct {
	id *identity_manager.Manager

	mu             sync.RWMutex
	policy         security.Policy
	fused          *transport_manager.FusedTransport
	router         *message_router.Router
	ttlSec         uint16
	routerDefaults message_router.Options
}

type Options struct {
	Policy   security.Policy
	TTL      uint16
	Validate func(message_router.Message) bool

	Router message_router.Options
}

func New(id *identity_manager.Manager, transports []transport_manager.Transport, opts Options) (*Controller, error) {
	if id == nil {
		return nil, errors.New("identity manager required")
	}
	if len(transports) == 0 {
		return nil, errors.New("at least one transport required")
	}
	if opts.TTL == 0 {
		opts.TTL = 8
	}

	fused, err := transport_manager.NewFused(transport_manager.Options{
		Profile:    opts.Policy.TransportProfile(),
		Multipath:  opts.Policy.MultipathEnabled(),
		Transports: transports,
	})
	if err != nil {
		return nil, err
	}

	ro := opts.Policy.ApplyRouterDefaults(opts.Router)
	if opts.Validate != nil {
		ro.Validate = opts.Validate
	}
	r, err := message_router.NewRouter(fused, id.Current, ro)
	if err != nil {
		return nil, err
	}

	return &Controller{
		id:             id,
		policy:         opts.Policy,
		fused:          fused,
		router:         r,
		ttlSec:         opts.TTL,
		routerDefaults: ro,
	}, nil
}

func (c *Controller) Router() *message_router.Router { return c.router }

func (c *Controller) Run(ctx context.Context) error { return c.router.Run(ctx) }

func (c *Controller) SetOnMessage(fn func(message_router.Message)) { c.router.SetOnMessage(fn) }

func (c *Controller) SetPolicy(p security.Policy, now time.Time) {
	c.mu.Lock()
	c.policy = p
	f := c.fused
	r := c.router
	base := c.routerDefaults
	c.mu.Unlock()

	if f != nil {
		f.SetProfile(p.TransportProfile())
		f.SetMultipath(p.MultipathEnabled())
	}
	if r != nil {
		ro := p.ApplyRouterDefaults(base)
		r.SetCoverTraffic(ro.CoverTrafficEvery, ro.CoverTrafficSize, now)
	}
}

func (c *Controller) Send(now time.Time, payload []byte, ttlSec uint16, pref transport_selector.Preference) (message_router.MessageID, error) {
	c.mu.RLock()
	p := c.policy
	f := c.fused
	defTTL := c.ttlSec
	c.mu.RUnlock()

	if ttlSec == 0 {
		ttlSec = defTTL
	}
	if pref == 0 {
		pref = p.PreferenceForPayload(len(payload))
	}
	var id message_router.MessageID
	var err error
	f.WithPreference(pref, func() {
		id, err = c.router.Send(now, payload, ttlSec)
	})
	return id, err
}
