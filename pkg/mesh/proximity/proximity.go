package proximity

import (
	"context"
	"errors"
	"time"

	"github.com/kaveh/sunlionet-agent/core/mesh/mesh_controller"
	"github.com/kaveh/sunlionet-agent/core/mesh/security"
	"github.com/kaveh/sunlionet-agent/core/proximity/identity_manager"
	"github.com/kaveh/sunlionet-agent/core/proximity/message_router"
	"github.com/kaveh/sunlionet-agent/core/transport/transport_manager"
	"github.com/kaveh/sunlionet-agent/pkg/runtimecfg"
)

type Options struct {
	Policy security.Policy
	Router message_router.Options
	TTL    uint16
	Mode   runtimecfg.RuntimeMode
	Buffer int
}

type Mesh struct {
	mode runtimecfg.RuntimeMode
	ctrl *mesh_controller.Controller
	in   chan []byte
}

func New(id *identity_manager.Manager, transports []transport_manager.Transport, opts Options) (*Mesh, error) {
	if id == nil {
		return nil, errors.New("mesh: identity manager required")
	}
	if len(transports) == 0 {
		return nil, errors.New("mesh: at least one transport required")
	}
	if opts.Mode == "" {
		opts.Mode = runtimecfg.ModeReal
	}
	if opts.Buffer <= 0 {
		opts.Buffer = 256
	}
	ctrl, err := mesh_controller.New(id, transports, mesh_controller.Options{
		Policy: opts.Policy,
		TTL:    opts.TTL,
		Router: opts.Router,
	})
	if err != nil {
		return nil, err
	}
	m := &Mesh{
		mode: opts.Mode,
		ctrl: ctrl,
		in:   make(chan []byte, opts.Buffer),
	}
	ctrl.SetOnMessage(func(msg message_router.Message) {
		cp := append([]byte(nil), msg.Payload...)
		select {
		case m.in <- cp:
		default:
		}
	})
	return m, nil
}

func (m *Mesh) Run(ctx context.Context) error {
	if m == nil || m.ctrl == nil {
		<-ctx.Done()
		return ctx.Err()
	}
	return m.ctrl.Run(ctx)
}

func (m *Mesh) Broadcast(data []byte) error {
	if m == nil || m.ctrl == nil {
		return errors.New("mesh: nil")
	}
	_, err := m.ctrl.Send(time.Now(), data, 0, 0)
	return err
}

func (m *Mesh) Receive(ctx context.Context) ([]byte, error) {
	if m == nil {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case b := <-m.in:
		return b, nil
	}
}

func (m *Mesh) RuntimeMode() runtimecfg.RuntimeMode {
	if m == nil {
		return runtimecfg.ModeReal
	}
	return m.mode
}
