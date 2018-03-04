package portal

import (
	"github.com/SentimensRG/ctx"
	"github.com/SentimensRG/ctx/sigctx"
	"github.com/pkg/errors"
)

// Cfg is a base configuration struct
type Cfg struct {
	ctx.Doner
	Size         int
	DropWhenFull bool
}

// Async returns true if the Portal is buffered
func (c Cfg) Async() bool { return c.Size > 0 }

// MakePortal is for protocol implementations
func MakePortal(cfg Cfg, p Protocol) Portal {
	var cancel func()
	var d ctx.Doner
	if cfg.Doner == nil {
		d = sigctx.New()
	}

	cfg.Doner, cancel = ctx.WithCancel(d)
	return newPortal(p, cfg, cancel)
}

type portal struct {
	Cfg
	cancel func()

	id    ID
	proto Protocol
	ready bool

	chSend chan *Message
	chRecv chan *Message

	ProtocolSendHook
	ProtocolRecvHook
}

func newPortal(p Protocol, cfg Cfg, cancel func()) *portal {
	var ptl = new(portal)

	ptl.Cfg = cfg
	ptl.cancel = cancel
	ptl.id = NewID()
	ptl.proto = p
	ptl.chSend = make(chan *Message, cfg.Size)
	ptl.chRecv = make(chan *Message, cfg.Size)

	if i, ok := interface{}(p).(ProtocolSendHook); ok {
		ptl.ProtocolSendHook = i.(ProtocolSendHook)
	}
	if i, ok := interface{}(p).(ProtocolRecvHook); ok {
		ptl.ProtocolRecvHook = i.(ProtocolRecvHook)
	}

	p.Init(ptl)

	return ptl
}

func (p *portal) setRunning() {
	p.ready = true
	ctx.Defer(p, func() { p.ready = false })
}

func (p *portal) Connect(addr string) (err error) {
	if boundEP, err := addrTable.Lookup(addr); err != nil {
		err = errors.Wrap(err, addr)
	} else {
		boundEP.ConnectEndpoint(p)
		p.ConnectEndpoint(boundEP)
		p.setRunning()
	}

	return
}

func (p *portal) Bind(addr string) (err error) {
	if err := addrTable.Assign(addr, p); err != nil {
		err = errors.Wrap(err, addr)
	} else {
		p.setRunning()
	}

	return
}

func (p *portal) TrySend(v interface{}) (err error) {
	defer func() { err = recover().(error) }()
	p.Send(v)
	return
}

func (p *portal) TryRecv() (v interface{}, err error) {
	defer func() { err = recover().(error) }()
	v = p.Recv()
	return
}

func (p *portal) Send(v interface{}) {
	if !p.ready {
		panic(errors.New("send to disconnected portal"))
	}

	msg := NewMsg()
	msg.Value = v

	p.SendMsg(msg)

	if p.Async() {
		go msg.wait()
	} else {
		msg.wait()
	}
}

func (p *portal) Recv() (v interface{}) {
	if !p.ready {
		panic(errors.New("recv from disconnected portal"))
	}

	if msg := p.RecvMsg(); msg != nil {
		v = msg.Value
		msg.Free()
	}

	return
}

func (p *portal) SendMsg(msg *Message) {
	if (p.ProtocolSendHook != nil) && !p.SendHook(msg) {
		msg.Free()
		return // drop msg silently
	}

	if p.Cfg.DropWhenFull {
		select {
		case p.chSend <- msg:
		default:
			msg.Free()
		}
	} else {
		select {
		case p.chSend <- msg:
		case <-p.Done():
			msg.Free()
		}
	}

}

func (p *portal) RecvMsg() *Message {
	for {
		select {
		case msg := <-p.chRecv:
			if (p.ProtocolRecvHook != nil) && !p.SendHook(msg) {
				msg.Free()
			} else {
				return msg
			}
		case <-p.Done():
			return nil
		}
	}
}

func (p *portal) Close() { p.cancel() }

// Implement Endpoint
func (p *portal) ID() ID { return p.id }

// func (p *portal) Notify(msg *Message)          { p.chRecv <- msg }
// func (p *portal) Announce() *Message           { return <-p.chSend }
func (p *portal) Signature() ProtocolSignature { return p.proto }

// Implement ProtocolSocket
func (p *portal) SendChannel() <-chan *Message  { return p.chSend }
func (p *portal) RecvChannel() chan<- *Message  { return p.chRecv }
func (p *portal) CloseChannel() <-chan struct{} { return p.Done() }

// gc manages the lifecycle of an endpoint in the background
func (p *portal) ConnectEndpoint(ep Endpoint) {
	p.proto.AddEndpoint(ep)
	ctx.Defer(ctx.Link(p, ep), func() { p.proto.RemoveEndpoint(ep) })
}
