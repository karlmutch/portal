package req

import (
	"sync"

	"github.com/lthibault/portal"
	proto "github.com/lthibault/portal/protocol/core"
)

type req struct {
	sync.Mutex

	prtl portal.ProtocolPortal
	n    proto.Neighborhood
}

func (r *req) Init(prtl portal.ProtocolPortal) {
	r.prtl = prtl
	r.n = proto.NewNeighborhood()
}

func (r *req) startSending(pe proto.PeerEndpoint) {

	// NB: Because this function is only called when an endpoint is
	// added, we can reasonably safely cache the channels -- they won't
	// be changing after this point.

	sq := r.prtl.SendChannel()
	cq := r.prtl.CloseChannel()

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

func (r *req) startReceiving(ep portal.Endpoint) {
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

	rq := r.prtl.RecvChannel()
	cq := r.prtl.CloseChannel()

	for msg = ep.Announce(); msg != nil; msg = ep.Announce() {
		select {
		case <-cq:
			break
		case rq <- msg:
		}
	}
}

func (*req) Number() uint16     { return portal.ProtoReq }
func (*req) PeerNumber() uint16 { return portal.ProtoRep }
func (*req) Name() string       { return "req" }
func (*req) PeerName() string   { return "rep" }

func (r *req) AddEndpoint(ep portal.Endpoint) {
	portal.MustBeCompatible(r, ep.Signature())

	pe := proto.NewPeerEP(ep)

	r.n.SetPeer(ep.ID(), pe)

	go r.startSending(pe)
	go r.startReceiving(ep)
}

func (r *req) RemoveEndpoint(ep portal.Endpoint) { r.n.DropPeer(ep.ID()) }

// New allocates a Portal using the REQ protocol
func New(cfg portal.Cfg) portal.Portal {
	return portal.MakePortal(cfg.Ctx, &req{})
}
