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
	t.Parallel()
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
	t.Parallel()
	var ini func(R00) R01
	var invoke func(R02) R03
	require.NoError(t, Sequence(t.Name(),
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
			return "05", "06", R03(r04) + "D"
		},
	).Bind(&invoke, &ini))
	assert.Equal(t, R01("00A"), ini("00"))
	assert.Equal(t, R03("00AB1C1D"), invoke("02"))
}

func TestReorderChaos(t *testing.T) {
	t.Parallel()
	var invoke func(R00) R10
	require.NoError(t, Sequence("outer", Reorder(Sequence(t.Name(),
		func(r03 R03, r04 R04) (R04, R06) { return "<" + R04(r03) + ">A1", "<" + R06(r03) + ">A1" },
		func(r00 R00, r01 R01) R00 { return "<" + r00 + R00(r01) + ">B" },
		func() R01 { return "<C>" },
		func(r05 R05) (R06, R04) { return "<" + R06(r05) + ">D1", "<" + R04(r05) + ">D2" },
		func(inner func(R07) R08, r06 R06) (R09, R10) {
			x := inner("<" + R07(r06) + ">E1")
			return "<" + R09(x) + ">E2", "<" + R10(x) + ">E3"
		},
		func(inner func(R06) R09, r05 R05) R10 {
			return "<" + R10(inner("<"+R06(r05)+">F1")) + ">F2"
		},
		func(r02 R02) R03 { return "<" + R03(r02) + ">G" },
		func(r04 R04, r06 R06) R07 { return "<" + R07(r04) + "><" + R07(r06) + ">H" },
		func(r03 R03) R06 { return "<" + R06(r03) + ">I" },
		func(r00 R00) R05 { return "<" + R05(r00) + ">J" },
		func(r01 R01, r07 R07) R08 { return "<" + R08(r01) + "><" + R08(r07) + ">K" },
	))).Bind(&invoke, nil))
	r := invoke("invoke")
	t.Log("got:", r)
	assert.NotEmpty(t, r)
	t.Log("because every value gets wrapped with <> if we have an empty <> that indicates an unwrapped value")
	assert.NotContains(t, r, "<>")
}

func TestReorderUnused(t *testing.T) {
	t.Parallel()
	var invoke func(R00) R08
	var dd *Debugging
	require.NoError(t, Sequence("outer", Reorder(Sequence(t.Name(),
		func(r03 R03, r04 R04) (R04, R06) { return "<" + R04(r03) + ">A1", "<" + R06(r03) + ">A1" },
		func(r00 R00, r01 R01) R00 { return "<" + r00 + R00(r01) + ">B" },
		func() R01 { return "<C>" },
		func(r05 R05) (R06, R04) { return "<" + R06(r05) + ">D1", "<" + R04(r05) + ">D2" },
		func(inner func(R07) R08, r06 R06) (R09, R10) {
			x := inner("<" + R07(r06) + ">E1")
			return "<" + R09(x) + ">E2", "<" + R10(x) + ">E3"
		},
		func(inner func(R06) R09, r05 R05) R10 {
			return "<" + R10(inner("<"+R06(r05)+">F1")) + ">F2"
		},
		func(r02 R02) R03 { return "<" + R03(r02) + ">G" },
		func(r04 R04, r06 R06) R07 { return "<" + R07(r04) + "><" + R07(r06) + ">H" },
		func(r03 R03) R06 { return "<" + R06(r03) + ">I" },
		func(r00 R00) R05 { return "<" + R05(r00) + ">J" },
		func(r04 R04, d *Debugging) R08 {
			dd = d
			return "<" + R08(r04) + ">K"
		},
	))).Bind(&invoke, nil))
	r := invoke("invoke")
	t.Log("got:", r)
	assert.NotEmpty(t, r)
	t.Log("because every value gets wrapped with <> if we have an empty <> that indicates an unwrapped value")
	assert.NotContains(t, r, "<>")
	t.Log(strings.Join(dd.IncludeExclude, "\n"))
	if assert.NotNil(t, dd) {
		assert.Less(t, len(dd.Included)+3, len(dd.IncludeExclude))
	}
}

func TestReorderOverride(t *testing.T) {
	t.Parallel()
	var dd *Debugging
	seq1 := Sequence("example",
		Shun(func() string {
			assert.Fail(t, "fallback used")
			return "fallback default"
		}),
	)
	seq2 := Sequence("later inputs",
		Reorder(func() string {
			return "override value"
		}),
	)
	require.NoError(t, Run("combination",
		seq1,
		seq2,
		func(s string, d *Debugging) {
			dd = d
			assert.Equal(t, "override value", s)
		},
	))
	if t.Failed() {
		t.Log(dd.Trace)
	}
}
