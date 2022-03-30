package nject

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type R00 string
type R01 string
type R02 string
type R03 string
type R04 string
type R05 string
type R06 string
type R07 string
type R08 string
type R09 string
type R10 string

func TestReorderSimpleMove(t *testing.T) {
	assert.NoError(t, Run(t.Name(),
		NotCacheable(func() R00 { return "00" }),
		func(r2 R02) R03 { return R03(r2) + "foo" },
		Reorder(func(r0 R00) R02 { return R02(r0) + "bar" }), // has to move up one
		func(r03 R03) { assert.Equal(t, "00barfoo", string(r03)) },
	))
}
