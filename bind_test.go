package nject

// TODO: Test MustBindSimple
// TODO: Test MustBindSimpleError
// TODO: write more examples
// TODO: do a bunch of bind init and invoke in parallel to exercise the locks
// TODO: test Then

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type (
	s0 string
	s1 string
	s2 string
	s3 string
	s4 string
	s5 string
	s6 string
	s7 string
	s8 string
	s9 string
)

type s2bypass string

type (
	s1prime string
	s2prime string
	s3prime string
	s4prime string
	s5prime string
	s6prime string
	s7prime string
	s8prime string
)

type a1 []*s1

type i1 interface {
	s1() s1
}

type i2 interface {
	s2() s2
}

type i3 interface {
	s3() s3
}

type i4 interface {
	s4() s4
}
type (
	ie      any
	i4prime interface {
		s4() s4
		s4prime() s4prime
	}
)

type i5 interface {
	s5() s5
}

type i6 interface {
	s6() s6
}

type i7 interface {
	s7() s7
}

type i8 interface {
	s8() s8
}

type i9 interface {
	s9() s9
}
type i2imp int

func (i2imp) s2() s2 { return "" }

const (
	s1Value s1 = "s1 value"
	s2Value s2 = "s2 value"
	s3Value s3 = "s3 value"
	s4Value s4 = "s4 value"
	s5Value s5 = "s5 value"
)

// TestInitInvokeInjections does a very basic test of
// providing values in init and invoke and seeing return
// values from init and invoke.   It also verifies
// the number of invocations for each item making sure
// about what is run during init and what is run
// for each invocation.
func TestInitInvokeInjections(t *testing.T) {
	wrapTest(t, func(t *testing.T) {
		counts := make(map[string]int)
		c := Sequence("TBF",
			Cacheable(func(s s0) s1 {
				counts["s1"]++
				assert.Equal(t, s0("s0 value"), s)
				return s1Value
			}),
			Cacheable(func(s s1) s2 {
				counts["s2"]++
				assert.Equal(t, s1Value, s)
				return s2Value
			}),
			Cacheable(func(s s3) s4 {
				// Nothing uses s4 so this doesn't run
				counts["s4"]++
				assert.Equal(t, s3Value, s)
				return s4Value
			}),
			Cacheable(func(s s2) s5 {
				counts["s5"]++
				assert.Equal(t, s2Value, s)
				return s5Value
			}),
		)
		var shouldWorkInit func(s0) s2
		var shouldWorkInvoke func(s3) s5
		err := c.Bind(&shouldWorkInvoke, &shouldWorkInit)
		require.NoError(t, err)
		assert.Equal(t, s2Value, shouldWorkInit("s0 value"))
		assert.Equal(t, s2Value, shouldWorkInit("ignored"))
		assert.Equal(t, s5Value, shouldWorkInvoke(s3Value))
		assert.Equal(t, s5Value, shouldWorkInvoke(s3Value))
		assert.Equal(t, s5Value, shouldWorkInvoke(s3Value))
		assert.Equal(t, 1, counts["s1"])
		assert.Equal(t, 1, counts["s2"])
		assert.Equal(t, 0, counts["s4"])
		assert.Equal(t, 3, counts["s5"])
	})
}

// TestRequired is deliberately identical to TestInitInvokeInjections
// except that it switches s4 from optional (and thus eliminated) to
// mandatory.
func TestRequired(t *testing.T) {
	wrapTest(t, func(t *testing.T) {
		counts := make(map[string]int)
		c := Sequence("TBF",
			Cacheable(func(s s0) s1 {
				t.Logf("s1 called with '%s'", s)
				counts["s1"]++
				assert.Equal(t, s0("s0 value"), s)
				return s1Value
			}),
			Cacheable(func(s s1) s2 {
				t.Logf("s2 called with '%s'", s)
				counts["s2"]++
				assert.Equal(t, s1Value, s)
				return s2Value
			}),
			Required(Cacheable(func(s s3) s4 {
				t.Logf("s4 called with '%s'", s)
				counts["s4"]++
				assert.Equal(t, s3Value, s)
				return s4Value
			})),
			Cacheable(func(s s2) s5 {
				t.Logf("s5 called with '%s'", s)
				counts["s5"]++
				assert.Equal(t, s2Value, s)
				return s5Value
			}),
		)
		var shouldWorkInit func(s0) s2
		var shouldWorkInvoke func(s3) s5
		err := c.Bind(&shouldWorkInvoke, &shouldWorkInit)
		require.NoError(t, err)

		assert.Equal(t, s2Value, shouldWorkInit("s0 value"))
		assert.Equal(t, s2Value, shouldWorkInit("ignored"))

		assert.Equal(t, s5Value, shouldWorkInvoke(s3Value))
		assert.Equal(t, s5Value, shouldWorkInvoke(s3Value))
		assert.Equal(t, s5Value, shouldWorkInvoke(s3Value))

		assert.Equal(t, 1, counts["s1"])
		assert.Equal(t, 1, counts["s2"])
		assert.Equal(t, 3, counts["s4"])
		assert.Equal(t, 3, counts["s5"])
	})
}

// TestInitDependencyOnUnavailableData is deliberately identical
// to TestInitInvokeInjections except that it switches s2 from cacheable
// to uncacheable and thus prevents its output from being available to
// init.
func TestInitDependencyOnUnavailableData(t *testing.T) {
	wrapTest(t, func(t *testing.T) {
		counts := make(map[string]int)
		c := Sequence("TBF",
			Cacheable(func(s s0) s1 {
				t.Logf("s1 called with '%s'", s)
				counts["s1"]++
				assert.Equal(t, s0("s0 value"), s)
				return s1Value
			}),
			Provide("s2", func(s s1) s2 {
				t.Logf("s2 called with '%s'", s)
				counts["s2"]++
				assert.Equal(t, s1Value, s)
				return s2Value
			}),
			Cacheable(func(s s3) s4 {
				t.Logf("s4 called with '%s'", s)
				counts["s4"]++
				assert.Equal(t, s3Value, s)
				return s4Value
			}),
			Cacheable(func(s s2) s5 {
				t.Logf("s5 called with '%s'", s)
				counts["s5"]++
				assert.Equal(t, s2Value, s)
				return s5Value
			}),
		)
		var shouldWorkInit func(s0) s2
		var shouldWorkInvoke func(s3) s5
		err := c.Bind(&shouldWorkInvoke, &shouldWorkInit)
		require.Error(t, err)
		assert.Panics(t, func() {
			MustBind(c, &shouldWorkInvoke, &shouldWorkInit)
		})
	})
}

func TestHoistToStaticFromSubCollections(t *testing.T) {
	wrapTest(t, func(t *testing.T) {
		counts := make(map[string]int)
		c := Sequence("TBF",
			Cacheable(func(s s0) s1 {
				counts["s1"]++
				assert.Equal(t, s0("s0 value"), s)
				return s1Value
			}),
			Cacheable(func(s s1) s2 {
				counts["s2"]++
				assert.Equal(t, s1Value, s)
				return s2Value
			}),
			Sequence("Inner",
				Cacheable(func(s s1) s6 {
					counts["s6"]++
					assert.Equal(t, s1Value, s)
					return "s6 value"
				}),
				// This is not static because it gets its input from invoke
				Required(Cacheable(func(s s3) s7 {
					counts["s7"]++
					assert.Equal(t, s3Value, s)
					return "s7 value"
				})),
				Provide("s8", func(s s1) s8 {
					counts["s8"]++
					assert.Equal(t, s1Value, s)
					return "s8 value"
				}),
			),
			Required(Cacheable(func(s s6) s9 {
				counts["s9"]++
				assert.Equal(t, s6("s6 value"), s)
				return "s9 value"
			})),
			Required(Cacheable(func(s s8) s4 {
				counts["s4"]++
				assert.Equal(t, s8("s8 value"), s)
				return s4Value
			})),
			Cacheable(func(s s2) s5 {
				counts["s5"]++
				assert.Equal(t, s2Value, s)
				return s5Value
			}),
		)
		var shouldWorkInit func(s0) s2
		var shouldWorkInvoke func(s3) s5
		err := c.Bind(&shouldWorkInvoke, &shouldWorkInit)
		require.NoError(t, err)
		assert.Equal(t, s2Value, shouldWorkInit("s0 value"))
		assert.Equal(t, s2Value, shouldWorkInit("ignored"))
		assert.Equal(t, s5Value, shouldWorkInvoke(s3Value))
		assert.Equal(t, s5Value, shouldWorkInvoke(s3Value))
		assert.Equal(t, s5Value, shouldWorkInvoke(s3Value))
		assert.Equal(t, 1, counts["s1"], "s1 static chain")
		assert.Equal(t, 1, counts["s2"], "s2 static chain")
		assert.Equal(t, 3, counts["s4"], "s4 invoke chain")
		assert.Equal(t, 3, counts["s5"], "s5 invoke chain")
		assert.Equal(t, 1, counts["s6"], "s6 static chain")
		assert.Equal(t, 3, counts["s7"], "s7 invoke chain")
		assert.Equal(t, 3, counts["s8"], "s8 invoke chain")
		assert.Equal(t, 1, counts["s9"], "s9 static chain")
	})
}

// TestLiteral does a very basic test of providing literal
// values.
func TestLiteral(t *testing.T) {
	wrapTest(t, func(t *testing.T) {
		counts := make(map[string]int)
		c := Sequence("TBF",
			s1Value,
			Cacheable(func(s s1) s2 {
				counts["s2"]++
				assert.Equal(t, s1Value, s)
				return s2Value
			}),
			Cacheable(func(s s3) s4 {
				// Nothing uses s4 so this doesn't run
				counts["s4"]++
				assert.Equal(t, s3Value, s)
				return s4Value
			}),
			Cacheable(func(s s2) s5 {
				counts["s5"]++
				assert.Equal(t, s2Value, s)
				return s5Value
			}),
		)
		var shouldWorkInit func(s0) s2
		var shouldWorkInvoke func(s3) s5
		MustBind(c, &shouldWorkInvoke, &shouldWorkInit)
		assert.Equal(t, s2Value, shouldWorkInit("s0 value"))
		assert.Equal(t, s2Value, shouldWorkInit("ignored"))
		assert.Equal(t, s5Value, shouldWorkInvoke(s3Value))
		assert.Equal(t, s5Value, shouldWorkInvoke(s3Value))
		assert.Equal(t, s5Value, shouldWorkInvoke(s3Value))
		assert.Equal(t, 1, counts["s2"])
		assert.Equal(t, 0, counts["s4"])
		assert.Equal(t, 3, counts["s5"])
	})
}

// TestMemoize validates that memoized values are cached globalls
func TestMemoize(t *testing.T) {
	wrapTest(t, func(t *testing.T) {
		expected := "s0 value"
		counts := make(map[string]int)
		c := Sequence("TBF",
			Memoize(func(s s0) s1 {
				counts["s1"]++
				assert.Equal(t, s0(expected), s)
				return s1("s1 value" + expected)
			}),
			Cacheable(func(s s1) s2 {
				counts["s2"]++
				assert.Equal(t, s1("s1 value"+expected), s)
				return s2Value + s2(string(s))
			}),
			Cacheable(func(s s2) s5 {
				counts["s5"]++
				assert.Equal(t, s2("s2 values1 value"+expected), s)
				return s5Value
			}),
		)
		var seqInit1 func(s0) s2
		var seqInvoke1 func(s3) s5
		err := c.Bind(&seqInvoke1, &seqInit1)
		require.NoError(t, err)

		var seqInit2 func(s0) s2
		var seqInvoke2 func(s3) s5
		err = Sequence("S2", c).Bind(&seqInvoke2, &seqInit2)
		require.NoError(t, err)

		var seqInit3 func(s0) s2
		var seqInvoke3 func(s3) s5
		err = Sequence("S3", c).Bind(&seqInvoke3, &seqInit3)
		require.NoError(t, err)

		assert.Equal(t, s2("s2 values1 values0 value"), seqInit1("s0 value"))
		assert.Equal(t, s2("s2 values1 values0 value"), seqInit1("ignored"))
		assert.Equal(t, s5Value, seqInvoke1(s3Value))
		assert.Equal(t, s5Value, seqInvoke1(s3Value))
		assert.Equal(t, s5Value, seqInvoke1(s3Value))

		expected = "s0 value #2"
		assert.Equal(t, s2("s2 values1 values0 value #2"), seqInit2("s0 value #2"))
		assert.Equal(t, s2("s2 values1 values0 value #2"), seqInit2("ignored"))
		assert.Equal(t, s5Value, seqInvoke2(s3Value))
		assert.Equal(t, s5Value, seqInvoke2(s3Value))
		assert.Equal(t, s5Value, seqInvoke2(s3Value))

		expected = "s0 value"
		assert.Equal(t, s2("s2 values1 values0 value"), seqInit3("s0 value"))
		assert.Equal(t, s2("s2 values1 values0 value"), seqInit3("ignored"))
		assert.Equal(t, s5Value, seqInvoke3(s3Value))
		assert.Equal(t, s5Value, seqInvoke3(s3Value))
		assert.Equal(t, s5Value, seqInvoke3(s3Value))

		assert.Equal(t, 2, counts["s1"])
		assert.Equal(t, 3, counts["s2"])
		assert.Equal(t, 9, counts["s5"])
	})
}

func errorIfNotEqual(a any, b any) error {
	if a != b {
		return fmt.Errorf("was expecting %v but got %v", a, b)
	}
	return nil
}

func terminalErrorSetup(t *testing.T) (map[string]int, *Collection) {
	counts := make(map[string]int)
	c := Sequence("TBF",
		Provide("S0", Cacheable(func(s s0) (s1, TerminalError, s1prime, s2bypass) {
			counts["s1"]++
			t.Logf("s1(s0=%s)", s)
			s2b := s2Value
			if s0("s0 value") != s {
				s2b = ""
			}
			return s1Value, errorIfNotEqual(s0("s0 value"), s), "s1 prime", s2bypass(s2b)
		})),
		Provide("S1", Cacheable(func(s s1) s2 {
			counts["s2"]++
			t.Logf("s2(s1=%s)", s)
			require.Equal(t, s1Value, s)
			return s2Value
		})),

		Desired(Provide("S9", func(inner func() s7) {
			counts["s9"]++
			t.Logf("s9()")
			_ = inner()
		})),
		Desired(Provide("S8", func(inner func() (error, s7, s7prime)) s7 {
			counts["s8"]++
			t.Logf("s8()")
			err, s, s7p := inner()
			t.Logf("inner returned:\n\terr=%s\n\ts=%s\n\ts7p=%s", err, s, s7p)
			if err != nil {
				return "error"
			}
			return s
		})),
		Provide("S4", func(s s3, s3p s3prime) (s4, TerminalError, s4prime) {
			counts["s4"]++
			t.Logf("s4(s3=%s, s3p=%s)", s, s3p)
			require.Equal(t, s3Value, s)
			return s4Value, errorIfNotEqual(s3prime("s3 prime"), s3p), "s4 prime"
		}),
		Provide("S5", func(s s4, sp s1prime) (s5, s5prime, TerminalError) {
			counts["s5"]++
			t.Logf("s5(s4=%s, s1p=%s)", s, sp)
			require.Equal(t, s4Value, s)
			return s5Value, "s5 prime", errorIfNotEqual(s1prime("s1 prime"), sp)
		}),
		Provide("S6", func(s s2, sp s2prime, s2b s2bypass) (TerminalError, s6, s6prime) {
			counts["s6"]++
			t.Logf("s6(s2=%s, s2p=%s, s2b=%s)", s, sp, s2b)
			require.Equal(t, s2(s2b), s)
			return errorIfNotEqual(s2prime("s2 prime"), sp), "s6 value", "s6 prime"
		}),
		Provide("S7", func(vs4 s4, s4p s4prime, vs5 s5, s5p s5prime, vs6 s6, s6p s6prime) (s7, s7prime) {
			counts["s7"]++
			t.Logf("s7(s4=%s, s4p=%s, s5=%s, s5p=%s, s6=%s, s6p=%s)", vs4, s4p, vs5, s5p, vs6, s6p)
			require.Equal(t, s4Value, vs4)
			require.Equal(t, s5Value, vs5)
			require.Equal(t, s6("s6 value"), vs6)
			require.Equal(t, s4prime("s4 prime"), s4p)
			require.Equal(t, s5prime("s5 prime"), s5p)
			require.Equal(t, s6prime("s6 prime"), s6p)
			return "s7 value", "s7 prime"
		}),
	)
	return counts, c
}

func TestTerminalErrorMultiBinding(t *testing.T) {
	wrapTest(t, func(t *testing.T) {
		counts, c := terminalErrorSetup(t)

		debugln("------------------------ bind with bind1Init ----------------")
		var bind1Init func(s0) (s1, s1prime, error)
		var bind1Invoke func(s3, s1prime, s2prime, s3prime) s7
		err := c.Bind(&bind1Invoke, &bind1Init)
		require.NoError(t, err)

		debugln("------------------------ bind with bind2Init ----------------")
		var bind2Init func(s0) (s1, s1prime, error)
		var bind2Invoke func(s3, s1prime, s2prime, s3prime) s7prime
		err = c.Bind(&bind2Invoke, &bind2Init)
		require.NoError(t, err)

		debugln("------------------------ call      bind1Init ----------------")
		bind1s1, bind1s1p, bind1e := bind1Init("s0 value")
		require.Equal(t, s1Value, bind1s1)
		require.Equal(t, s1prime("s1 prime"), bind1s1p)
		require.NoError(t, bind1e)

		debugln("------------------------ call bind1init (again) -------------")
		bind1s1, bind1s1p, bind1e = bind1Init("ignored")
		require.Equal(t, s1Value, bind1s1)
		require.Equal(t, s1prime("s1 prime"), bind1s1p)
		require.NoError(t, bind1e)

		debugln("------------------------ call      bind2Init ----------------")
		bind2s1, bind2s1p, bind2e := bind2Init("not s0 value")
		require.Equal(t, s1Value, bind2s1)
		require.Equal(t, s1prime("s1 prime"), bind2s1p)
		require.Error(t, bind2e)

		debugln("------------------------ call bind2init (again) -------------")
		bind2s1, bind2s1p, bind2e = bind2Init("s0 value")
		require.Equal(t, s1Value, bind2s1)
		require.Equal(t, s1prime("s1 prime"), bind2s1p)
		require.Error(t, bind2e)

		debugln("------------------------ call invoke1 -----------------------")
		debugln("invoke1(s3=s3 value, s1p=s1 prime, s2p=s2 prime, s3p=s3 prime)")
		require.Equal(t, s7("s7 value"), bind1Invoke(s3Value, "s1 prime", "s2 prime", "s3 prime"))

		debugln("------------------------ call invoke1 -----------------------")
		debugln("invoke1(s3=s3 value, s1p=s1 other, s2p=s2 prime, s3p=s3 prime)")
		require.Equal(t, s7("error"), bind1Invoke(s3Value, "s1 other", "s2 prime", "s3 prime"))

		debugln("------------------------ call invoke1 -----------------------")
		debugln("invoke1(s3=s3 value, s1p=s1 prime, s2p=s2 other, s3p=s3 prime)")
		require.Equal(t, s7("error"), bind1Invoke(s3Value, "s1 prime", "s2 other", "s3 prime"))

		debugln("------------------------ call invoke1 -----------------------")
		debugln("invoke1(s3=s3 value, s1p=s1 prime, s2p=s2 prime, s3p=s3 other)")
		require.Equal(t, s7("error"), bind1Invoke(s3Value, "s1 prime", "s2 prime", "s3 other"))

		debugln("------------------------ call invoke2 -----------------------")
		debugln("invoke2(s3=s3 value, s1p=s1 prime, s2p=s2 prime, s3p=s3 prime)")
		require.Equal(t, s7prime("s7 prime"), bind2Invoke(s3Value, "s1 prime", "s2 prime", "s3 prime"))

		debugln("------------------------ call invoke2 -----------------------")
		debugln("invoke2(s3=s3 value, s1p=s1 other, s2p=s2 prime, s3p=s3 prime)")
		require.Equal(t, s7prime(""), bind2Invoke(s3Value, "s1 other", "s2 prime", "s3 prime"))

		debugln("------------------------ call invoke2 -----------------------")
		debugln("invoke2(s3=s3 value, s1p=s1 prime, s2p=s2 other, s3p=s3 prime)")
		require.Equal(t, s7prime(""), bind2Invoke(s3Value, "s1 prime", "s2 other", "s3 prime"))

		debugln("------------------------ call invoke2 -----------------------")
		debugln("invoke2(s3=s3 value, s1p=s1 prime, s2p=s2 prime, s3p=s3 other)")
		require.Equal(t, s7prime(""), bind2Invoke(s3Value, "s1 prime", "s2 prime", "s3 other"))

		assert.Equal(t, 2, counts["s1"], "count for S1")
		assert.Equal(t, 1, counts["s2"], "count for S2")
		assert.Equal(t, 8, counts["s4"], "count for S4")
		assert.Equal(t, 6, counts["s5"], "count for S5")
		assert.Equal(t, 4, counts["s6"], "count for S6")
		assert.Equal(t, 2, counts["s7"], "count for S7")
		assert.Equal(t, 8, counts["s8"], "count for S8")
		assert.Equal(t, 8, counts["s9"], "count for S9")
	})
}

var ternbbte = Sequence("ternbbte",
	// this func is required because it receives from the final func
	Provide("WRAPPER", func(func() (s2, error), s1) {}),
	Provide("TERMINAL", func() TerminalError { return nil }),
	Provide("FINAL", func() (s2, error) { return "", nil }),
)

func TestErrorRequiredNotBlockedByTerminalErrorPass(t *testing.T) {
	wrapTest(t, func(t *testing.T) {
		var i1 func(s1) error
		assert.NoError(t, ternbbte.Bind(&i1, nil))
	})
}

func TestErrorRequiredNotBlockedByTerminalErrorFail(t *testing.T) {
	wrapTest(t, func(t *testing.T) {
		var i2 func(s2) error
		// This fails because the wrapper must be included
		// because it receives the s2 but it cannot because
		// there is no source of s1.
		assert.Error(t, ternbbte.Bind(&i2, nil))
	})
}
