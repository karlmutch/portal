package pub

import (
	"sync"

	"github.com/lthibault/portal"
	proto "github.com/lthibault/portal/proto"
)

// Protocol implementing PUB
type Protocol struct {
	ptl portal.ProtocolPortal
	n   proto.Neighborhood
}

// Init the Protocol
func (p *Protocol) Init(ptl portal.ProtocolPortal) {
	p.ptl = ptl
	p.n = proto.NewNeighborhood()
	go p.startSending()
}

func (p Protocol) startSending() {
	cq := p.ptl.CloseChannel()
	sq := p.ptl.SendChannel()

	var wg sync.WaitGroup

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

			m, done := p.n.RMap()
			wg.Add(len(m))

			for _, peer := range m {
				go func(p portal.Endpoint) {
					p.RecvChannel() <- msg.Ref()
					wg.Done()
				}(peer)
			}

			done()
			wg.Wait()
			msg.Free()
		}
	}
}

func (p Protocol) AddEndpoint(ep portal.Endpoint) {
	proto.MustBeCompatible(p, ep.Signature())
	p.n.SetPeer(ep.ID(), ep)
}

func (p Protocol) RemoveEndpoint(ep portal.Endpoint) { p.n.DropPeer(ep.ID()) }

func (Protocol) Number() uint16     { return proto.Pub }
func (Protocol) PeerNumber() uint16 { return proto.Sub }

// New allocates a portal using the PUB protocol
func New(cfg portal.Cfg) portal.WriteOnly {
	return struct{ portal.WriteOnly }{portal.MakePortal(cfg, &Protocol{})} // write guard
}
