package pair

import (
	"sync"

	"github.com/lthibault/portal"
	"github.com/lthibault/portal/proto"
)

// Protocol implementing PAIR
type Protocol struct {
	sync.Mutex
	ptl  portal.ProtocolPortal
	peer proto.PeerEndpoint
}

// Init the Protocol
func (p *Protocol) Init(ptl portal.ProtocolPortal) { p.ptl = ptl }

func (p *Protocol) AddEndpoint(ep portal.Endpoint) {
	proto.MustBeCompatible(p, ep.Signature())

	p.Lock()
	defer p.Unlock()

	if p.peer != nil { // we already have a conn, reject this one
		ep.Close()
	} else {
		p.peer = proto.NewPeerEP(ep)
		go p.startReceiving()
		go p.startSending()
	}
}

func (p *Protocol) RemoveEndpoint(ep portal.Endpoint) {
	p.Lock()
	defer p.Unlock()

	if peer := p.peer; peer != nil && peer.ID() == ep.ID() {
		ep.Close()
		p.peer = nil
		peer.Close()
	}
}

func (*Protocol) Number() uint16     { return proto.Pair }
func (*Protocol) Name() string       { return "pair" }
func (*Protocol) PeerNumber() uint16 { return proto.Pair }
func (*Protocol) PeerName() string   { return "pair" }

func (p *Protocol) startReceiving() {
	var msg *portal.Message
	defer func() {
		if msg != nil {
			msg.Free()
		}
		if r := recover(); r != nil {
			panic(r)
		}
	}()

	rq := p.ptl.RecvChannel()
	cq := p.ptl.CloseChannel()

	for msg = p.peer.Announce(); msg != nil; p.peer.Announce() {
		select {
		case rq <- msg:
		case <-cq:
			return
		}
	}
}

func (p *Protocol) startSending() {
	sq := p.ptl.SendChannel()
	cq := p.ptl.CloseChannel()
	pcq := p.peer.Done()

	// This is pretty easy because we have only one peer at a time.
	// If the peer goes away, drop the message on the floor.
	var msg *portal.Message
	for {
		select {
		case <-pcq:
			return
		case <-cq:
			return
		case msg = <-sq:
			if msg == nil {
				sq = p.ptl.SendChannel()
			} else {
				p.peer.Notify(msg) // may panic
			}
		}
	}
}

// New allocates a Portal using the PAIR protocol
func New(cfg portal.Cfg) portal.Portal {
	return portal.MakePortal(cfg, &Protocol{})
}
