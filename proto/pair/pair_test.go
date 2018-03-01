package pair

import (
	"testing"

	"github.com/lthibault/portal"
	"github.com/stretchr/testify/assert"
	"golang.org/x/sync/errgroup"
)

const iter = 10000

func TestIntegration(t *testing.T) {
	p0 := New(portal.Cfg{})
	p1 := New(portal.Cfg{})

	assert.NoError(t, p0.Bind("/test/pair/integration"))
	assert.NoError(t, p1.Connect("/test/pair/integration"))

	var g errgroup.Group

	var l2r, r2l int

	// Test Left to Right
	g.Go(func() (err error) {
		for i := 0; i < iter; i++ {
			p0.Send(i)
		}
		return
	})

	g.Go(func() (err error) {
		for i := 0; i < iter; i++ {
			l2r = p1.Recv().(int)
		}
		return
	})

	// Test Right to Left
	g.Go(func() (err error) {
		for i := 0; i < iter; i++ {
			p1.Send(i)
		}
		return
	})

	g.Go(func() (err error) {
		for i := 0; i < iter; i++ {
			r2l = p0.Recv().(int)
		}
		return
	})

	assert.NoError(t, g.Wait())
	assert.Equal(t, iter-1, l2r, "left to right:  expected %d, got %d", iter-1, l2r)
	assert.Equal(t, iter-1, r2l, "right to left:  expected %d, got %d", iter-1, r2l)
}
