package nject

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		NotCacheable(func() R00 {
			t.Log("1. -> R00 ")
			return "00"
		}),
		func(r2 R02) R03 {
			t.Log("2. R02 -> R03")
			return R03(r2) + "foo"
		},
		Reorder(func(r0 R00) R02 {
			t.Log("3. R00 -> R02")
			return R02(r0) + "bar"
		}), // has to move up one
		func(r03 R03, d *Debugging) {
			t.Log("4. final")
			t.Log(strings.Join(d.IncludeExclude, "\n"))
			assert.Equal(t, "00barfoo", string(r03))
		},
	))
}

func TestReorderWrappers(t *testing.T) {
	var ini func(R00) R01
	var invoke func(R02) R03
	require.NoError(t, Sequence("test",
		Memoize(func(r00 R00) (R01, R04) { return R01(r00) + "A", R04(r00) + "A" }),
		Reorder(func(inner func(r04 R04) R06, r04 R04) R05 {
			return R05(inner(r04+"C1")) + "C2"
		}),
		Reorder(func(inner func(r04 R04) R05, r04 R04) {
			_ = inner(r04 + "B1")
		}),
		func(r02 R02, r04 R04) (R05, R06, R03) {
			assert.Equal(t, R02("02"), r02)
			assert.Equal(t, R04("00AB1C1"), r04)
			return "05", "06", "03"
		},
	).Bind(&invoke, &ini))
	assert.Equal(t, R01("00A"), ini("00"))
	assert.Equal(t, R03("03"), invoke("02"))
}

/*
	func(r00 R00) R04 { return R04(r00) + "r04" },
	func(r04 R04) (R01, R05) { return R01(r04), R05(r04) },
	Reorder(func(inner func(R06, R07) R08, r02 R02, r09 R09) R03 {
		return R03(inner(R06(r02), R07(r04)))
	}),
	Reorder(func(r04 R04) R09 { return R09(r04) },
	Reorder(func(inner func(R10) R08,
*/
