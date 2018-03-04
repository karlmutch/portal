package sub

import (
	"sync"

	"github.com/lthibault/portal"
	proto "github.com/lthibault/portal/proto"
	"github.com/pkg/errors"
)

// Topic evaluates whether a value should be broadcast to a topic
type Topic interface {
	Match(interface{}) bool
}

// TopicFunc turns a `func(interface{}) bool` into a Topic
type TopicFunc func(interface{}) bool

// Match returns true if the interface matches the topic rule
func (f TopicFunc) Match(v interface{}) bool { return f(v) }

var (
	// TopicAll includes all values received on the SUB portal
	TopicAll TopicFunc = func(interface{}) bool { return true }

	// TopicNone does not accept anything
	TopicNone TopicFunc = func(interface{}) (ok bool) { return }

	// TopicNotNil includes all values that are not nil
	TopicNotNil TopicFunc = func(v interface{}) bool { return v != nil }
)

type subscription struct {
	sync.RWMutex
	t []Topic
}

func (s *subscription) Match(v interface{}) (matched bool) {
	s.RLock()
	for _, t := range s.t {
		if matched = t.Match(v); matched {
			break
		}
	}
	s.RUnlock()
	return
}

func (s *subscription) Subscribe(t Topic) (err error) {
	s.Lock()
	defer s.Unlock()

	for _, tpc := range s.t {
		if tpc == t {
			return errors.New("already subscribed to topic")
		}
	}

	s.t = append(s.t, t)
	return
}

func (s *subscription) Unsubscribe(t Topic) {
	s.Lock()
	defer s.Unlock()

	for i, tpc := range s.t {
		if tpc == t {
			s.t[i] = s.t[len(s.t)-1]
			s.t = s.t[:len(s.t)-1]
		}
	}
}

// Protocol implementing SUB
type Protocol struct {
	ptl  portal.ProtocolPortal
	subs *subscription
}

func (p *Protocol) Init(ptl portal.ProtocolPortal) {
	p.ptl = ptl
	p.subs = &subscription{t: make([]Topic, 0)}
}

func (p Protocol) startReceiving(ep portal.Endpoint) {
	rq := p.ptl.RecvChannel()
	cq := p.ptl.CloseChannel()

	for msg := range ep.SendChannel() {
		if p.subs.Match(msg.Value) {
			select {
			case rq <- msg:
			case <-cq:
				return
			}
		} else {
			msg.Free()
		}
	}
}

func (Protocol) Number() uint16     { return proto.Sub }
func (Protocol) PeerNumber() uint16 { return proto.Pub }

func (Protocol) RemoveEndpoint(portal.Endpoint) {}
func (p Protocol) AddEndpoint(ep portal.Endpoint) {
	proto.MustBeCompatible(p, ep.Signature())
	go p.startReceiving(ep)
}

func (p Protocol) Subscribe(t Topic) error { return p.subs.Subscribe(t) }
func (p Protocol) Unsubscribe(t Topic)     { p.subs.Unsubscribe(t) }

// Portal adds the (Un)Subscribe methods to portal.ReadOnly
type Portal interface {
	portal.ReadOnly
	Subscribe(Topic) error
	Unsubscribe(Topic)
}

// New allocates a portal using the SUB protocol
func New(cfg portal.Cfg) Portal {
	s := &Protocol{}
	return struct {
		portal.ReadOnly
		*Protocol
	}{
		ReadOnly: portal.MakePortal(cfg, s),
		Protocol: s,
	}
}
