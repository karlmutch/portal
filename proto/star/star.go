package star

import (
	"sync"

	"github.com/SentimensRG/ctx"
	"github.com/lthibault/portal"
	"github.com/lthibault/portal/proto"
)

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
	cq := ctx.Link(ctx.Lift(s.star.ptl.CloseChannel()), s)

	for msg := range s.SendChannel() {
		select {
		case <-cq:
			msg.Free()
			return
		default:
			s.star.broadcast(msg, &s)
		}
	}
}

type starNeighborhood struct {
	sync.RWMutex
	epts map[portal.ID]*starEP
}

func (n *starNeighborhood) RMap() (map[portal.ID]*starEP, func()) {
	n.RLock()
	return n.epts, n.RUnlock
}

func (n *starNeighborhood) SetPeer(id portal.ID, se *starEP) {
	n.Lock()
	n.epts[id] = se
	n.Unlock()
}

func (n *starNeighborhood) GetPeer(id portal.ID) (se *starEP, ok bool) {
	n.RLock()
	se, ok = n.epts[id]
	n.RUnlock()
	return
}

func (n *starNeighborhood) DropPeer(id portal.ID) {
	n.Lock()
	be := n.epts[id]
	delete(n.epts, id)
	n.Unlock()

	if be != nil {
		be.Close()
	}
}

// Protocol implementing STAR
type Protocol struct {
	ptl portal.ProtocolPortal
	n   *starNeighborhood
}

// Init the protocol (called by portal)
func (p *Protocol) Init(ptl portal.ProtocolPortal) {
	p.ptl = ptl
	p.n = &starNeighborhood{epts: make(map[portal.ID]*starEP)}
	go p.startSending()
}

func (p Protocol) startSending() {

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

			p.broadcast(msg, nil)
		}
	}
}

func (p Protocol) broadcast(msg *portal.Message, sender *starEP) {
	defer msg.Free()

	// Relay to all neighbors
	m, done := p.n.RMap()
	defer done()
	for _, se := range m {
		if sender == se {
			continue
		}

		se.sendMsg(msg.Ref())
	}

	if sender != nil { // the message originates from a remote peer; receive it.
		select {
		case p.ptl.RecvChannel() <- msg.Ref():
		case <-p.ptl.CloseChannel():
		}
	}
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

// New allocates a portal using the STAR protocol
func New(cfg portal.Cfg) portal.Portal {
	return portal.MakePortal(cfg, &Protocol{})
}
