package portal

import (
	"sync"

	"github.com/SentimensRG/ctx"
	"github.com/pkg/errors"
)

var transport = trans{lookup: make(map[string]*binding)}

type (
	transporter interface {
		Connect(addr string) error
		Bind(addr string) error
		Close()
	}

	connector interface {
		ctx.Doner
		GetEndpoint() Endpoint
		Connect(Endpoint)
	}

	listener interface {
		ctx.Doner
		Listen() <-chan Endpoint
	}
)

type binding struct {
	ctx.Doner
	addr  string
	cxns  chan Endpoint // incomming connections
	bound Endpoint
}

func newBinding(d ctx.Doner, addr string, e Endpoint) *binding {
	return &binding{Doner: d, addr: addr, cxns: make(chan Endpoint), bound: e}
}

func (b binding) Addr() string            { return b.addr }
func (b binding) GetEndpoint() Endpoint   { return b.bound }
func (b binding) Connect(ep Endpoint)     { b.cxns <- ep }
func (b binding) Listen() <-chan Endpoint { return b.cxns }
func (b binding) Close()                  { close(b.cxns) }

type trans struct {
	sync.RWMutex
	lookup map[string]*binding
}

func (t *trans) GetConnector(addr string) (c connector, ok bool) {
	t.RLock()
	c, ok = t.lookup[addr]
	t.RUnlock()
	return
}

func (t *trans) GetListener(d ctx.Doner, addr string, e Endpoint) (listener, error) {
	t.Lock()
	defer t.Unlock()

	b := newBinding(d, addr, e)
	if _, exists := t.lookup[b.Addr()]; exists {
		return nil, errors.Errorf("transport exists at %s", b.Addr())
	}

	t.lookup[b.Addr()] = b
	ctx.Defer(d, t.garbageCollect(b))

	return b, nil
}

func (t *trans) garbageCollect(b *binding) func() {
	return func() {
		b.Close()
		t.Lock()
		delete(t.lookup, b.Addr())
		t.Unlock()
	}
}
