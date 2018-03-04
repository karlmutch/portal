package star

import (
	"testing"

	"github.com/lthibault/portal"
	"github.com/stretchr/testify/assert"
)

const integrationAddr = "/test/star/integration"

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
			bP.Recv()
			panic("bP should not recv its own messages")
		}()

		assert.True(t, cP[0].Recv().(bool))
		assert.True(t, cP[1].Recv().(bool))
		assert.True(t, cP[2].Recv().(bool))
	})

	// t.Run("SendConn", func(t *testing.T) {
	// 	go cP[0].Send(true)

	// 	var wg sync.WaitGroup
	// 	wg.Add(len(cP))
	// 	for i, p := range append(cP[1:], bP) {
	// 		go func(i int, p portal.Portal) {
	// 			assert.True(t, p.Recv().(bool), "%d was not true", i)
	// 			wg.Done()
	// 		}(i, p)
	// 	}

	// 	ch := make(chan struct{})
	// 	go func() {
	// 		wg.Wait()
	// 		close(ch)
	// 	}()

	// 	select {
	// 	case <-ch:
	// 	case <-time.After(time.Millisecond * 100):
	// 		t.Error("symmetric many-to-many property violated")
	// 	}
	// })
}
