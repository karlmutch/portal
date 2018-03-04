package bus

import (
	"fmt"
	"log"
	"sync"
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
		var wg sync.WaitGroup

		ch := make(chan struct{})
		chSync := make(chan struct{})
		go func() {
			wg.Add(nPtls - 1) // nPtls - one bind portal
			close(chSync)
			wg.Wait()
			ch <- struct{}{}
		}()

		for i, p := range cP {
			go func(i int, p portal.Portal) {
				<-chSync
				log.Printf("connPortal %d recved %b", i, p.Recv())
				wg.Done()
			}(i, p)
		}

		go func() {
			<-chSync
			bP.Send(true)
		}()

		select {
		case <-ch:
		case <-time.After(time.Millisecond * 100):
			t.Error("at least one portal did not receive the value sent by bP")
		}
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
