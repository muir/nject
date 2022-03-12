package nject

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetCallbackInvokeOnly(t *testing.T) {
	wrapTest(t, func(t *testing.T) {
		var cb func(s1, s2) s3
		setCallback := func(h func(s1, s2) s3) {
			cb = h
		}
		require.NoError(t,
			Sequence("SCBT",
				func(x s2, y s1) s3 {
					return s3(y) + s3(x)
				},
			).SetCallback(setCallback))
		assert.Equal(t, s3("foobar"), cb("foo", "bar"))
	})
}

func TestSetCallbackWithInit(t *testing.T) {
	wrapTest(t, func(t *testing.T) {
		var cb func(s2) s3
		var cbInit func(s1)
		setCallback := func(h func(s2) s3, i func(s1)) {
			cb = h
			cbInit = i
		}
		require.NoError(t,
			Sequence("SCBT",
				func(x s2, y s1) s3 {
					return s3(y) + s3(x)
				},
			).SetCallback(setCallback))
		cbInit("foo")
		assert.Equal(t, s3("foobar"), cb("bar"))
	})
}
