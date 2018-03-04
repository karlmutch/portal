package rep

import (
	"sync"

	"github.com/lthibault/portal"
	proto "github.com/lthibault/portal/proto"
)

const eqBufSize = 8

// Protocol implementing REP
type Protocol struct {
	sync.Mutex
	ptl portal.ProtocolPortal
	n   proto.Neighborhood
}

func (p *Protocol) Init(ptl portal.ProtocolPortal) {
	p.ptl = ptl
	p.n = proto.NewNeighborhood()
}

func (p *Protocol) startServing(ep portal.Endpoint) {
	var msg *portal.Message
	defer func() {
		if msg != nil {
			msg.Free()
		}
		if r := recover(); r != nil {
			msg.Free()
			panic(r)
		}
	}()

	cq := p.ptl.CloseChannel()
	rq := p.ptl.RecvChannel()
	sq := p.ptl.SendChannel()

	for msg = ep.Announce(); msg != nil; msg = ep.Announce() {
		p.Lock()

		select {
		case <-cq:
			p.Unlock()
			return
		case rq <- msg:
			select {
			case <-cq:
				p.Unlock()
				return
			case <-pe.Done():
				msg.Free()
				p.Unlock()
				continue
			case msg = <-sq:
				if msg == nil {
					sq = p.ptl.SendChannel()
					p.Unlock()
				} else {
					pe.Notify(msg)
					p.Unlock()
				}
			}
		}
	}
}

func (*Protocol) Number() uint16     { return proto.Rep }
func (*Protocol) PeerNumber() uint16 { return proto.Req }

func (p *Protocol) AddEndpoint(ep portal.Endpoint) {
	proto.MustBeCompatible(p, ep.Signature())

	p.n.SetPeer(ep.ID(), ep)

	go p.startServing(ep)
}

func (p *Protocol) RemoveEndpoint(ep portal.Endpoint) { p.n.DropPeer(ep.ID()) }

// New allocates a new REP portal
func New(cfg portal.Cfg) portal.Portal {
	return portal.MakePortal(cfg, &Protocol{})
}
