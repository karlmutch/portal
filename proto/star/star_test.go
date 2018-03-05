package star

import (
	"testing"

	"github.com/lthibault/portal"
	"github.com/stretchr/testify/assert"
)

const integrationAddr = "/test/bus/integration"

func initPtls(t *testing.T, nPtls int) (bP portal.Portal, cP []portal.Portal) {
	ptls := make([]portal.Portal, nPtls)
	for i := range ptls {
		ptls[i] = New(portal.Cfg{})
	}

	bP, cP = ptls[0], ptls[1:len(ptls)]

	assert.NoError(t, bP.Bind(integrationAddr))

	for _, p := range cP {
		assert.NoError(t, p.Connect(integrationAddr))
	}

	return
}

func closeAll(p ...portal.Portal) {
	for _, p := range p {
		p.Close()
	}
}

func TestIntegration(t *testing.T) {
	t.Run("SendBind", func(t *testing.T) {
		bP, cP := initPtls(t, 4)
		defer closeAll(append(cP, bP)...)

		go bP.Send(true)

		go func() {
			if bP.Recv() != nil {
				panic("sender should not should not recv its own messages")
			}
		}()

		assert.True(t, cP[0].Recv().(bool))
		assert.True(t, cP[1].Recv().(bool))
		assert.True(t, cP[2].Recv().(bool))
	})

	t.Run("SendConn", func(t *testing.T) {
		bP, cP := initPtls(t, 4)
		defer closeAll(append(cP, bP)...)

		go cP[0].Send(true)

		go func() {
			if cP[0].Recv() != nil {
				panic("sender should not recv its own messages")
			}
		}()

		assert.True(t, bP.Recv().(bool))
		assert.True(t, cP[1].Recv().(bool))
		assert.True(t, cP[2].Recv().(bool))
	})
}
