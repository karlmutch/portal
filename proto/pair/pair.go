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
	peer portal.Endpoint
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
		p.peer = ep
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
func (*Protocol) PeerNumber() uint16 { return proto.Pair }

func (p *Protocol) startReceiving() {
	rq := p.ptl.RecvChannel()
	cq := p.ptl.CloseChannel()

	for msg := range p.peer.SendChannel() {
		select {
		case rq <- msg:
		case <-cq:
			msg.Free()
			return
		}
	}
}

func (p *Protocol) startSending() {
	sq := p.ptl.SendChannel()
	cq := p.ptl.CloseChannel()

	prq := p.peer.RecvChannel()
	pcq := p.peer.Done()

	// This is pretty easy because we have only one peer at a time.
	// If the peer goes away, drop the message on the floor.
	for {
		select {
		case <-cq:
			return
		case msg, ok := <-sq:
			if ok {
				select {
				case prq <- msg:
				case <-pcq:
					msg.Free()
					return
				}
			}
		}
	}
}

// New allocates a Portal using the PAIR protocol
func New(cfg portal.Cfg) portal.Portal {
	return portal.MakePortal(cfg, &Protocol{})
}
