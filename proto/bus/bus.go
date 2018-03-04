package bus

import (
	"sync"

	"github.com/SentimensRG/ctx"
	"github.com/lthibault/portal"
	proto "github.com/lthibault/portal/proto"
)

type busEP struct {
	portal.Endpoint
	q   chan *portal.Message
	bus *Protocol
}

func (b busEP) close() {
	b.Endpoint.Close()
	close(b.q)
}

func (b busEP) sendMsg(msg *portal.Message) {
	select {
	case b.q <- msg:
	case <-b.Done():
		msg.Free()
	}
}

func (b busEP) startSending() {
	rq := b.RecvChannel()
	cq := b.Done()
	for msg := range b.q {
		select {
		case rq <- msg:
		case <-cq:
			msg.Free()
			return
		}
	}
}

func (b busEP) startReceiving() {
	rq := b.bus.ptl.RecvChannel()
	cq := ctx.Link(ctx.Lift(b.bus.ptl.CloseChannel()), b)

	for msg := range b.SendChannel() {
		id := b.ID()
		msg.From = &id

		select {
		case <-cq:
			msg.Free()
			return
		case rq <- msg:
		}
	}
}

type busNeighborhood struct {
	sync.RWMutex
	epts map[portal.ID]*busEP
}

func (n *busNeighborhood) RMap() (map[portal.ID]*busEP, func()) {
	n.RLock()
	return n.epts, n.RUnlock
}

func (n *busNeighborhood) SetPeer(id portal.ID, be *busEP) {
	n.Lock()
	n.epts[id] = be
	n.Unlock()
}

func (n *busNeighborhood) GetPeer(id portal.ID) (b *busEP, ok bool) {
	n.RLock()
	b, ok = n.epts[id]
	n.RUnlock()
	return
}

func (n *busNeighborhood) DropPeer(id portal.ID) {
	n.Lock()
	be := n.epts[id]
	delete(n.epts, id)
	n.Unlock()

	if be != nil {
		be.Close()
	}
}

// Protocol implementing BUS
type Protocol struct {
	ptl portal.ProtocolPortal
	n   *busNeighborhood
}

// Init the protocol (called by portal)
func (p *Protocol) Init(ptl portal.ProtocolPortal) {
	p.ptl = ptl
	p.n = &busNeighborhood{epts: make(map[portal.ID]*busEP)}
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

			p.broadcast(msg)
		}
	}
}

func (p Protocol) broadcast(msg *portal.Message) {
	var wg sync.WaitGroup

	m, done := p.n.RMap() // get a read-locked map-view of the Neighborhood
	defer done()

	for id, peer := range m {
		// if there's a header, it means the msg was rebroadcast
		if msg.From != nil && id == *msg.From {
			continue
		}

		// proto.Neighborhood stores portal.Endpoints, so we must type-assert
		wg.Add(1)
		go func(bep *busEP) {
			bep.sendMsg(msg.Ref())
			wg.Done()
		}(peer)
	}

	wg.Wait()
}

func (p *Protocol) AddEndpoint(ep portal.Endpoint) {
	proto.MustBeCompatible(p, ep.Signature())

	pe := &busEP{Endpoint: ep, q: make(chan *portal.Message, 1), bus: p}
	p.n.SetPeer(ep.ID(), pe)
	go pe.startSending()
	go pe.startReceiving()
}

func (p Protocol) RemoveEndpoint(ep portal.Endpoint) { p.n.DropPeer(ep.ID()) }

func (Protocol) Number() uint16     { return proto.Bus }
func (Protocol) PeerNumber() uint16 { return proto.Bus }

// New allocates a portal using the BUS protocol
func New(cfg portal.Cfg) portal.Portal {
	return portal.MakePortal(cfg, &Protocol{})
}
