package star

import (
	"sync"

	"github.com/SentimensRG/ctx"
	"github.com/lthibault/portal"
	"github.com/lthibault/portal/proto"
)

type msgSender interface {
	sendMsg(*portal.Message)
}

type starEP struct {
	portal.Endpoint
	q    chan *portal.Message
	star *Protocol
}

func (s starEP) close() {
	s.Endpoint.Close()
	close(s.q)
}

func (s starEP) sendMsg(msg *portal.Message) {
	select {
	case s.q <- msg:
	case <-s.Done():
		msg.Free()
	}
}

func (s starEP) startSending() {
	rq := s.RecvChannel()
	cq := s.Done()
	for msg := range s.q {
		select {
		case rq <- msg:
		case <-cq:
			msg.Free()
			return
		}
	}
}

func (s starEP) startReceiving() {
	rq := s.star.ptl.RecvChannel()
	cq := ctx.Link(ctx.Lift(s.star.ptl.CloseChannel()), s)

	for msg := range s.SendChannel() {
		id := s.ID()
		msg.From = &id
		select {
		case <-cq:
			msg.Free()
			return
		case rq <- msg:
		}
	}
}

// Protocol implementing STAR
type Protocol struct {
	ptl portal.ProtocolPortal
	n   proto.Neighborhood
}

// Init the protocol (called by portal)
func (p *Protocol) Init(ptl portal.ProtocolPortal) {
	p.ptl = ptl
	p.n = proto.NewNeighborhood()
	go p.startSending()
}

func (p Protocol) startSending() {
	var wg sync.WaitGroup

	sq := p.ptl.SendChannel()
	cq := p.ptl.CloseChannel()

	for {
		select {
		case <-cq:
			return
		case msg, ok := <-sq:
			if !ok {
				// This should never happen.  If it does, the channels were not
				// closed in the correct order
				// TODO:  remove once tested & stable
				panic("ensure portal.Doner fires closes before chSend/chRecv")
			}

			p.broadcast(&wg, msg).Wait()
		}
	}
}

func (p Protocol) broadcast(wg *sync.WaitGroup, msg *portal.Message) (wgout *sync.WaitGroup) {
	m, done := p.n.RMap() // get a read-locked map-view of the Neighborhood
	defer done()

	for id, peer := range m {
		// if there's a header, it means the msg was rebroadcast
		if id == *msg.From {
			continue
		}

		// proto.Neighborhood stores portal.Endpoints, so we must type-assert
		go p.unicast(wg, peer.(msgSender), msg)
	}

	if msg.From != nil { // Grab a local copy and send it up
		select {
		case <-p.ptl.CloseChannel():
			msg.Free()
			return wg
		case p.ptl.RecvChannel() <- msg:
		}
	} else { // Not sending the message up, so let's release it
		msg.Free()
	}

	return wg
}

func (p Protocol) unicast(wg *sync.WaitGroup, s msgSender, msg *portal.Message) {
	wg.Add(1)
	s.sendMsg(msg.Ref())
	wg.Done()
}

func (p *Protocol) AddEndpoint(ep portal.Endpoint) {
	proto.MustBeCompatible(p, ep.Signature())

	pe := &starEP{Endpoint: ep, q: make(chan *portal.Message, 1), star: p}
	p.n.SetPeer(ep.ID(), pe)
	go pe.startSending()
	go pe.startReceiving()
}

func (p Protocol) RemoveEndpoint(ep portal.Endpoint) { p.n.DropPeer(ep.ID()) }

func (Protocol) Number() uint16     { return proto.Star }
func (Protocol) PeerNumber() uint16 { return proto.Star }
func (Protocol) Name() string       { return "star" }
func (Protocol) PeerName() string   { return "star" }
