package portal

import (
	"sync"
)

var (
	msgPool = messagePool{
		Pool: sync.Pool{New: func() interface{} { return new(Message).Ref() }},
	}
)

type messagePool struct{ sync.Pool }

func (pool *messagePool) Get() *Message    { return pool.Pool.Get().(*Message) }
func (pool *messagePool) Put(msg *Message) { go pool.put(msg) }
func (pool *messagePool) put(msg *Message) {
	msg.From = nil
	pool.Pool.Put(msg)
}

// Message wraps a value and sends it down the portal
type Message struct {
	wg    sync.WaitGroup
	From  *ID
	Value interface{}
}

// Free deallocates a message
func (m *Message) Free() { m.wg.Done() }

// Ref increments the reference count on the message.  Note that since the
// underlying message is actually shared, consumers must take care not
// to modify the message.  Applications should *NOT* make use of this
// function -- it is intended for Protocol, Transport and internal use only.
func (m *Message) Ref() *Message {
	m.wg.Add(1)
	return m
}

// Wait for the message to be delivered.  It MUST be called by the Send function.
func (m *Message) wait() {
	m.wg.Wait()
	msgPool.Put(m.Ref())
}

// NewMsg returns a message with a single refcount
func NewMsg() *Message { return msgPool.Get() }
