package star

import (
	"sync"

	"github.com/lthibault/portal"
	"github.com/lthibault/portal/proto"
)

type starEP struct {
	portal.Endpoint
	q chan *portal.Message
	p Protocol
}

func (s starEP) sendMsg(cq <-chan struct{}, m *portal.Message) {
	select {
	case s.Endpoint.RecvChannel() <- m:
	case <-cq:
		m.Free()
		return
	}
}

func (s starEP) Close() {
	close(s.q)
	s.Endpoint.Close()
}

// bottom sender (sends to endpoint)
func (s starEP) sender() {
	cq := s.Done()
	for m := range s.q {
		s.sendMsg(cq, m)
	}
}

// bottom receiver (receives from endpoint [-> to proto])
func (s *starEP) recver() {
	for m := range s.Endpoint.SendChannel() {
		s.p.broadcast(m, s)
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

func (n *starNeighborhood) SetPeer(id portal.ID, sep *starEP) {
	n.Lock()
	n.epts[id] = sep
	n.Unlock()
}

func (n *starNeighborhood) GetPeer(id portal.ID) (p *starEP, ok bool) {
	n.RLock()
	p, ok = n.epts[id]
	n.RUnlock()
	return
}

func (n *starNeighborhood) DropPeer(id portal.ID) {
	n.Lock()
	sep := n.epts[id]
	delete(n.epts, id)
	n.Unlock()

	if sep != nil {
		sep.Close()
	}
}

// Protocol implementing STAR
type Protocol struct {
	ptl portal.ProtocolPortal
	n   *starNeighborhood
}

// Init the protocol
func (p *Protocol) Init(ptl portal.ProtocolPortal) {
	p.ptl = ptl
	p.n = &starNeighborhood{epts: make(map[portal.ID]*starEP)}
	// go p.startSending()
}

func (p Protocol) broadcast(m *portal.Message, sender *starEP) {
	var wg sync.WaitGroup

	epts, done := p.n.RMap()
	wg.Add(len(epts))
	for _, sep := range epts {
		if sender == sep {
			continue
		}

		m.Ref()
		go func(peer *starEP) {
			select {
			case peer.q <- m:
			case <-p.ptl.CloseChannel():
				m.Free()
			}
			wg.Done()
		}(sep)
	}
	done()

	if sender != nil {
		p.recvLocal(m)
	}

	wg.Wait()
}

func (p Protocol) recvLocal(m *portal.Message) {
	select {
	case p.ptl.RecvChannel() <- m:
	case <-p.ptl.CloseChannel():
		m.Free()
	}
}

// func (p Protocol) startSending() {
// 	cq := p.ptl.CloseChannel()
// 	sq := p.ptl.SendChannel()

// 	for {
// 		select {
// 		case <-cq:
// 			return
// 		case m, ok := <-sq:
// 			if !ok {
// 				sq = p.ptl.SendChannel()
// 			}
// 			p.broadcast(m, nil)
// 		}
// 	}
// }

// AddEndpoint to the Portal
func (p Protocol) AddEndpoint(ep portal.Endpoint) {
	sep := &starEP{
		Endpoint: ep,
		p:        p,
		q:        make(chan *portal.Message, 1),
	}

	p.n.SetPeer(ep.ID(), sep)
	go sep.sender()
	go sep.recver()
}

// RemoveEndpoint from the Portal
func (p Protocol) RemoveEndpoint(ep portal.Endpoint) { p.n.DropPeer(ep.ID()) }

// Number identifies the protocol
func (Protocol) Number() uint16 { return proto.Star }

// PeerNumber identifies the peer's protocol
func (Protocol) PeerNumber() uint16 { return proto.Star }

// New protocol implementing STAR
func New(cfg portal.Cfg) portal.Portal {
	return portal.MakePortal(cfg, &Protocol{})
}
