package bus

import (
	"fmt"
	"testing"
	"time"

	"github.com/lthibault/portal"
	"github.com/stretchr/testify/assert"
)

const integrationAddr = "/test/bus/integration"

func TestIntegration(t *testing.T) {
	const nPtls = 4

	ptls := make([]portal.Portal, nPtls)
	for i := range ptls {
		ptls[i] = New(portal.Cfg{})
	}

	bP, cP := ptls[0], ptls[1:len(ptls)]

	assert.NoError(t, bP.Bind(integrationAddr))

	for _, p := range cP {
		assert.NoError(t, p.Connect(integrationAddr))
	}

	t.Run("SendBind", func(t *testing.T) {
		go bP.Send(true)
		go func() {
			if bP.Recv() != nil {
				panic("bP should not recv its own messages")
			}
		}()

		assert.True(t, cP[0].Recv().(bool))
		assert.True(t, cP[1].Recv().(bool))
		assert.True(t, cP[2].Recv().(bool))
	})

	t.Run("SendConn", func(t *testing.T) {
		for i, p := range cP {
			t.Run(fmt.Sprintf("SendPortal%d", i), func(t *testing.T) {
				go p.Send(true)

				bindCh := make(chan struct{})
				connCh := make(chan struct{})

				go func() {
					_ = bP.Recv().(bool)
					close(bindCh)
				}()

				for _, p := range cP {
					go func(p portal.Portal) {
						_ = p.Recv().(bool)
						close(connCh)
					}(p)
				}

				// bind should get it
				select {
				case <-bindCh:
				case <-time.After(time.Millisecond):
					t.Error("bound portal did not recv message")
				}

				// others should NOT get it
				select {
				case <-connCh:
					t.Error("at least one connected portal erroneously recved a message")
				case <-time.After(time.Millisecond * 10):
				}
			})
		}
	})
}
