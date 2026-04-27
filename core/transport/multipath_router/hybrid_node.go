package multipath_router

import (
	"context"
	"time"

	"github.com/kaveh/sunlionet-agent/core/proximity/identity_manager"
	"github.com/kaveh/sunlionet-agent/core/proximity/message_router"
	"github.com/kaveh/sunlionet-agent/core/transport/transport_manager"
	"github.com/kaveh/sunlionet-agent/core/transport/transport_selector"
)

type Node struct {
	Fused  *transport_manager.FusedTransport
	Router *message_router.Router
	ID     *identity_manager.Manager
}

type Options struct {
	Profile   transport_selector.Profile
	Multipath bool

	Router message_router.Options
}

func NewNode(id *identity_manager.Manager, transports []transport_manager.Transport, opts Options) (*Node, error) {
	fused, err := transport_manager.NewFused(transport_manager.Options{
		Profile:    opts.Profile,
		Multipath:  opts.Multipath,
		Transports: transports,
	})
	if err != nil {
		return nil, err
	}
	r, err := message_router.NewRouter(fused, id.Current, opts.Router)
	if err != nil {
		return nil, err
	}
	return &Node{Fused: fused, Router: r, ID: id}, nil
}

func (n *Node) Run(ctx context.Context) error {
	return n.Router.Run(ctx)
}

func (n *Node) Send(now time.Time, payload []byte, ttlSec uint16, pref transport_selector.Preference) (message_router.MessageID, error) {
	var id message_router.MessageID
	var err error
	n.Fused.WithPreference(pref, func() {
		id, err = n.Router.Send(now, payload, ttlSec)
	})
	return id, err
}
