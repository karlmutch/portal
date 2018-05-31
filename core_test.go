package portal

import (
	"testing"
	"time"

	"github.com/SentimensRG/ctx"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"golang.org/x/sync/errgroup"
)

var ( // test type constraints
	_ Portal = &portal{}
)

func TestMkPortal(t *testing.T) {
	proto := mockProto{}

	t.Run("DonerAssignment", func(t *testing.T) {
		d, cancel := ctx.WithCancel(ctx.C(make(chan struct{})))
		defer cancel()

		cfg := Cfg{Doner: d}
		ptl := newPortal(proto, cfg, cancel)
		assert.Equal(t, d, ptl.Doner, "Doner instance passed to Cfg not assigned to portal")
	})

	t.Run("Sync", func(t *testing.T) {
		d, cancel := ctx.WithCancel(ctx.C(make(chan struct{})))
		defer cancel()

		cfg := Cfg{Doner: d}
		ptl := newPortal(proto, cfg, cancel)

		assert.Equal(t, 0, ptl.Size, "configuration error: size should default to 0, got %d", ptl.Size)
		assert.False(t, ptl.Async(), "configuration error: portal is asynchronous")
	})

	t.Run("Async", func(t *testing.T) {
		d, cancel := ctx.WithCancel(ctx.C(make(chan struct{})))
		defer cancel()

		cfg := Cfg{Doner: d, Size: 1}
		ptl := newPortal(proto, cfg, cancel)

		assert.Equal(t, 1, ptl.Size, "configuration error: expected size=1, got %d", ptl.Size)
		assert.True(t, ptl.Async(), "configuration error: portal is asynchronous")
	})
}

func TestTransportIntegration(t *testing.T) {
	// BINDING PORTAL
	bindEPAdded := make(chan Endpoint)
	bindEPRemoved := make(chan Endpoint)

	bindProto := mockProto{
		epAdded:   bindEPAdded,
		epRemoved: bindEPRemoved,
	}

	dBind, dCancel := ctx.WithCancel(ctx.C(make(chan struct{})))

	cfg := Cfg{Doner: dBind}

	bindP := newPortal(bindProto, cfg, dCancel)

	// CONNECTING PORTAL
	connEPAdded := make(chan Endpoint)
	connEPRemoved := make(chan Endpoint)

	connProto := mockProto{
		epAdded:   connEPAdded,
		epRemoved: connEPRemoved,
	}

	dConn, cCancel := ctx.WithCancel(ctx.C(make(chan struct{})))

	cfg = Cfg{Doner: dConn}

	connP := newPortal(connProto, cfg, cCancel)

	// BIND
	assert.NoError(t, bindP.Bind("/XYZ"))

	t.Run("EndpointTransaction", func(t *testing.T) {

		t.Run("EndpointExchange", func(t *testing.T) {
			var g errgroup.Group

			g.Go(func() (err error) {
				select {
				case <-connEPAdded:
				case <-time.After(time.Millisecond * 100):
					err = errors.New("connEP timeout")
				}
				return
			})

			g.Go(func() (err error) {
				select {
				case <-bindEPAdded:
				case <-time.After(time.Millisecond * 100):
					err = errors.New("bindEP timeout")
				}
				return
			})

			// CONNECT:  THIS IS WHEN ENDPOINTS SHOULD BE *ADDED*
			assert.NoError(t, connP.Connect("/XYZ"))

			// TEST IF ENDPOINTS WERE RECEIVED
			assert.NoError(t, g.Wait())
		})

		t.Run("EndpointGC", func(t *testing.T) {
			var g errgroup.Group

			g.Go(func() (err error) {
				select {
				case <-connEPRemoved:
				case <-time.After(time.Millisecond * 100):
					err = errors.New("connEP timeout")
				}
				return
			})

			g.Go(func() (err error) {
				select {
				case <-bindEPRemoved:
				case <-time.After(time.Millisecond * 100):
					err = errors.New("bindEP timeout")
				}
				return
			})

			go bindP.Close()
			go connP.Close()

			// TEST IF ENDPOINTS WERE RECEIVED
			assert.NoError(t, g.Wait())
		})
	})
}

func mkSendRecvTestPortal(p Protocol, size int) (*portal, func()) {
	d, cancel := ctx.WithCancel(ctx.C(make(chan struct{})))
	cfg := Cfg{Doner: d, Size: size}
	ptl := newPortal(p, cfg, cancel)
	return ptl, cancel
}

func TestSendRecvClose(t *testing.T) {
	p := mockProtoExt{
		onSend: func(*Message) bool { return true },
		onRecv: func(*Message) bool { return true },
	}
	pDrop := mockProtoExt{
		onSend: func(*Message) bool { return false },
		onRecv: func(*Message) bool { return false },
	}

	t.Run("SendMsg", func(t *testing.T) {
		t.Run("Drop", func(t *testing.T) {
			ptl, _ := mkSendRecvTestPortal(pDrop, 1)
			defer ptl.Close()

			assert.NoError(t, ptl.Bind("/alpha"))

			msg := NewMsg()
			defer msg.wait()

			ptl.SendMsg(msg)
			select {
			case <-ptl.chSend:
				t.Error("value SHOULD NOT have been sent")
				msg.Free()
			default:
			}
		})

		t.Run("DontDrop", func(t *testing.T) {
			ptl, _ := mkSendRecvTestPortal(p, 1)
			defer ptl.Close()

			assert.NoError(t, ptl.Bind("/bravo"))

			msg := NewMsg()
			defer msg.wait()
			msg.Value = true

			ptl.SendMsg(msg)
			select {
			case m := <-ptl.chSend:
				if m.Value != msg.Value {
					t.Errorf("unexpected value in message (got %v, expected %v)", m.Value, msg.Value)
				}
				msg.Free()
			default:
				t.Error("value SHOULD have been sent")
			}
		})
	})

	t.Run("RecvMsg", func(t *testing.T) {
		t.Run("Drop", func(t *testing.T) {
			ptl, _ := mkSendRecvTestPortal(pDrop, 1)
			defer ptl.Close()

			assert.NoError(t, ptl.Bind("/charlie"))

			msg := NewMsg()
			defer msg.wait()
			ptl.chRecv <- msg

			ch := make(chan *Message)
			go func() { ch <- ptl.RecvMsg() }()

			select {
			case <-ch:
				t.Error("message SHOULD NOT have been sent")
				msg.Free()
			case <-time.After(time.Microsecond * 1):
			}
		})

		t.Run("DontDrop", func(t *testing.T) {
			ptl, _ := mkSendRecvTestPortal(p, 1)
			defer ptl.Close()

			assert.NoError(t, ptl.Bind("/bravo"))

			msg := NewMsg()
			defer msg.wait()
			msg.Value = true
			ptl.chRecv <- msg

			ch := make(chan *Message)
			go func() { ch <- ptl.RecvMsg() }()

			select {
			case m := <-ch:
				if m.Value.(bool) != msg.Value.(bool) {
					t.Errorf("unexpected value in message (expected %v, got %v)", msg.Value, m.Value)
				}
				msg.Free()
			case <-time.After(time.Microsecond * 1):
				t.Error("message SHOULD have been sent")
			}
		})
	})

	// t.Run("Send", func(t *testing.T) {
	// 	ptl, _ := mkSendRecvTestPortal(p, 1)
	// 	if err := ptl.Bind("/delta"); err != nil {
	// 		t.Errorf("failed to bind: %s", err)
	// 	}
	// 	defer ptl.Close()

	// 	ptl.Send(true)

	// 	select {
	// 	case msg := <-ptl.chSend:
	// 		if msg.refcnt != 1 {
	// 			t.Errorf("unexpected refcount in message (expected 1, got %d)", msg.refcnt)
	// 		} else if v := msg.Value.(bool); !v {
	// 			t.Errorf("unexpected value in message (expected true, got %v)", v)
	// 		}
	// 	default:
	// 		t.Error("no message in send channel")
	// 	}

	// })

	// t.Run("Recv", func(t *testing.T) {
	// 	ptl, _ := mkSendRecvTestPortal(p, 1)
	// 	if err := ptl.Bind("/echo"); err != nil {
	// 		t.Errorf("failed to bind: %s", err)
	// 	}
	// 	defer ptl.Close()

	// 	m := NewMsg()
	// 	m.Value = true
	// 	ptl.chRecv <- m

	// 	if v := ptl.Recv(); !v.(bool) {
	// 		t.Errorf("unexpected value in message (expected true, got %v)", v)
	// 	}

	// 	ptl.chRecv <- nil
	// 	_ = ptl.Recv() // make sure this doesn't panic from nil-ptr deref
	// })

	// t.Run("Close", func(t *testing.T) {
	// 	ptl, _ := mkSendRecvTestPortal(mockProto{}, 1)
	// 	ptl.Close()

	// 	select {
	// 	case <-ptl.Done():
	// 	default:
	// 		t.Error("call to close did not fire protocol's Doner")
	// 	}
	// })

}
