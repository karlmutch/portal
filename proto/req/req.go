package req

import (
	"github.com/lthibault/portal"
	proto "github.com/lthibault/portal/proto"
)

// Protocol implementing REQ
type Protocol struct {
	ptl portal.ProtocolPortal
	n   proto.Neighborhood
}

func (p *Protocol) Init(ptl portal.ProtocolPortal) {
	p.ptl = ptl
	p.n = proto.NewNeighborhood()
}

func (p Protocol) startSending(pe proto.PeerEndpoint) {

	// NB: Because this function is only called when an endpoint is
	// added, we can reasonably safely cache the channels -- they won't
	// be changing after this point.

	sq := p.ptl.SendChannel()
	cq := p.ptl.CloseChannel()

	var msg *portal.Message
	for {
		select {
		case msg = <-sq:
		case <-cq:
			return
		case <-pe.Done():
			return
		}

		pe.Notify(msg)
	}
}

func (p Protocol) startReceiving(ep portal.Endpoint) {
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

	rq := p.ptl.RecvChannel()
	cq := p.ptl.CloseChannel()

	for msg = ep.Announce(); msg != nil; msg = ep.Announce() {
		select {
		case <-cq:
			break
		case rq <- msg:
		}
	}
}

func (Protocol) Number() uint16     { return proto.Req }
func (Protocol) PeerNumber() uint16 { return proto.Rep }

func (p Protocol) AddEndpoint(ep portal.Endpoint) {
	proto.MustBeCompatible(p, ep.Signature())

	pe := proto.NewPeerEP(ep)

	p.n.SetPeer(ep.ID(), pe)

	go p.startSending(pe)
	go p.startReceiving(ep)
}

func (p Protocol) RemoveEndpoint(ep portal.Endpoint) { p.n.DropPeer(ep.ID()) }

// New allocates a Portal using the REQ protocol
func New(cfg portal.Cfg) portal.Portal {
	return portal.MakePortal(cfg, &Protocol{})
}
