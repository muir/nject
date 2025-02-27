package nject

// TODO: test MustConsume on terminal injector

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testSeq = Cacheable(Sequence("TBF",
	func(s s0) s1 {
		if s0("s0 value") != s {
			panic("s1")
		}
		return "s1 value"
	},
	func(s s1) s2 {
		if s1("s1 value") != s {
			panic("s2")
		}
		return "s2 value"
	},
	func(s s3) s4 {
		if s3("s3 value") != s {
			panic("s4")
		}
		return "s4 value"
	},
	func(s s2) s5 {
		if s2("s2 value") != s {
			panic("s5")
		}
		return "s5 value"
	}))

// TestRun verifies that run works.
func TestRunWorks(t *testing.T) {
	wrapTest(t, func(t *testing.T) {
		called := false
		require.NoError(t, Run("run1",
			s3("s3 value"),
			s0("s0 value"),
			testSeq, func(s s5) {
				assert.Equal(t, s5("s5 value"), s)
				called = true
			}))
		assert.True(t, called)
	})
}

func TestRunMissingValue(t *testing.T) {
	wrapTest(t, func(t *testing.T) {
		called := false
		require.Error(t, Run("run1",
			s0("s0 value"),
			testSeq, func(s s4) {
				assert.Equal(t, s4("s4 value"), s)
				called = true
			}))
		assert.False(t, called)
	})
}

func TestRunReturnsErrorNoError(t *testing.T) {
	testRunReturnsError(t, nil)
}

func TestRunReturnsErrorError(t *testing.T) {
	testRunReturnsError(t, fmt.Errorf("an error"))
}

func testRunReturnsError(t *testing.T, e error) {
	wrapTest(t, func(t *testing.T) {
		called := false
		err := Run("run returns error",
			s3("s3 value"),
			s0("s0 value"),
			testSeq,
			func(s s5) error {
				assert.Equal(t, s5("s5 value"), s)
				called = true
				return e
			})
		if e == nil {
			require.NoError(t, err)
		} else {
			require.Error(t, err)
			assert.Equal(t, e.Error(), err.Error())
		}
		assert.True(t, called)
	})
}

func TestMustRun(t *testing.T) {
	wrapTest(t, func(t *testing.T) {
		t.Run("no panic", func(t *testing.T) {
			MustRun("run1",
				s3("s3 value"),
				s0("s0 value"),
				testSeq,
				func(s s4) {
					assert.Equal(t, s4("s4 value"), s)
				})
		})

		t.Run("panic", func(t *testing.T) {
			assert.Panics(t, func() {
				MustRun("run1",
					s0("s0 value"),
					testSeq,
					func(s s4) {
						assert.Equal(t, s4("s4 value"), s)
					})
			})
		})
	})
}

func TestNilLiterals(t *testing.T) {
	wrapTest(t, func(t *testing.T) {
		var intp *int
		called := false
		require.NoError(t, Run("test nil",
			intp,
			nil,
			func(ip *int) error {
				var ip2 *int
				assert.Equal(t, ip2, ip)
				called = true
				return nil
			}))
		assert.True(t, called)
	})
}

func TestUnusedLiteral(t *testing.T) {
	wrapTest(t, func(t *testing.T) {
		var intp *int
		called := false
		require.NoError(t, Run("test unused",
			intp,
			"seven",
			func(s string) error {
				assert.Equal(t, "seven", s)
				called = true
				return nil
			}))
		assert.True(t, called)
	})
}

func testWrapperFuncs(t *testing.T) (*int, Provider, Provider, Provider) {
	var callCount int
	return &callCount,
		Provide("S2SYNC", func(inner func() s2, s2p s2prime) {
			assert.Equal(t, s2(s2p), inner())
		}),
		Required(Provide("S1S2", func(inner func(s0) s1, s s0) s2 {
			return s2(inner(s+s0("bar")) + s1("foo"))
		})),
		Provide("S0S1", func(s s0) s1 {
			assert.Equal(t, s0("bazbar"), s)
			callCount++
			return s1(s)
		})
}

func TestWrappersRunError(t *testing.T) {
	wrapTest(t, func(t *testing.T) {
		_, _, s1s2, s0s1 := testWrapperFuncs(t)
		require.Error(t, Run("tw2",
			s2prime("bazbarfoo"),
			s0("baz"),
			// s2sync not included, so nobody consumes s2 returned by s1s2
			s1s2,
			s0s1))
	})
}

func TestWrappersRunNoError(t *testing.T) {
	wrapTest(t, func(t *testing.T) {
		callCountP, s2sync, s1s2, s0s1 := testWrapperFuncs(t)
		require.NoError(t, Run("tw1",
			s2prime("bazbarfoo"),
			s0("baz"),
			s2sync,
			s1s2,
			s0s1))
		require.Equal(t, 1, *callCountP)
	})
}

func TestWrappersBindNoError(t *testing.T) {
	wrapTest(t, func(t *testing.T) {
		callCountP, _, s1s2, s0s1 := testWrapperFuncs(t)
		var tw3Invoke func(s0) s2
		require.NoError(t, Sequence("tw3", s1s2, s0s1).Bind(&tw3Invoke, nil))
		require.Equal(t, s2("bazbarfoo"), tw3Invoke("baz"))
		require.Equal(t, 1, *callCountP)
	})
}

func TestWrappersBindError(t *testing.T) {
	wrapTest(t, func(t *testing.T) {
		_, _, _, s0s1 := testWrapperFuncs(t)
		var tw4Invoke func(s0) s2
		require.Error(t, Sequence("tw4", s0s1).Bind(&tw4Invoke, nil))
	})
}

func TestEmpties(t *testing.T) {
	wrapTest(t, func(t *testing.T) {
		//nolint:testifylint // assert is okay
		assert.Error(t, Run("no final func"))
		//nolint:testifylint // assert is okay
		assert.Error(t, Run("no final func", nil))
		//nolint:testifylint // assert is okay
		assert.NoError(t, Run("no final func", func() {}))

		seq := Sequence("empty")
		//nolint:testifylint // assert is okay
		assert.Error(t, Run("no final func", seq))
		//nolint:testifylint // assert is okay
		assert.Error(t, Run("no final func", seq, nil))
		//nolint:testifylint // assert is okay
		assert.NoError(t, Run("no final func", seq, func() {}))

		seq2 := seq.Append("nothing")
		//nolint:testifylint // assert is okay
		assert.Error(t, Run("no final func", seq2))
		//nolint:testifylint // assert is okay
		assert.Error(t, Run("no final func", seq2, nil))
		//nolint:testifylint // assert is okay
		assert.NoError(t, Run("no final func", seq2, func() {}))

		seq3 := seq.Append("more nothing", Sequence("empty too"))
		//nolint:testifylint // assert is okay
		assert.Error(t, Run("no final func", seq3))
		//nolint:testifylint // assert is okay
		assert.Error(t, Run("no final func", seq3, nil))
		assert.NoError(t, Run("no final func", seq3, func() {}))
	})
}

func TestAppend(t *testing.T) {
	wrapTest(t, func(t *testing.T) {
		seq1 := Sequence("one",
			Cacheable(func(s s0) s1 {
				if s0("s0 value") != s {
					panic("s1")
				}
				return "s1 value"
			}),
			Cacheable(func(s s1) s2 {
				if s1("s1 value") != s {
					panic("s2")
				}
				return "s2 value"
			}))

		seq2 := Cacheable(Sequence("two",
			func(s s3) s4 {
				if s3("s3 value") != s {
					panic("s4")
				}
				return "s4 value"
			},
			func(s s2) s5 {
				if s2("s2 value") != s {
					panic("s5")
				}
				return "s5 value"
			}))

		assert.NoError(t, Run("x", s0("s0 value"),
			s3("s3 value"),
			s0("s0 value"),
			seq1,
			seq2,
			func(s s5) {
				assert.Equal(t, s5("s5 value"), s)
			}))

		assert.NoError(t, Run("x", s0("s0 value"),
			s3("s3 value"),
			s0("s0 value"),
			seq1.Append("three", seq2),
			func(s s5) {
				assert.Equal(t, s5("s5 value"), s)
			}))

		assert.NoError(t, Run("x", s0("s0 value"),
			s3("s3 value"),
			s0("s0 value"),
			Sequence("four", seq1, seq2),
			func(s s5) {
				assert.Equal(t, s5("s5 value"), s)
			}))
	})
}

func TestErrorStrings(t *testing.T) {
	wrapTest(t, func(t *testing.T) {
		invalid := func(int, func()) {}
		//nolint:testifylint // assert is okay
		assert.NoError(t, Run("one", func() {}))

		err := Run("one", invalid, func() {})
		if assert.Error(t, err) {
			assert.Contains(t, err.Error(), "one(0) ")
		}

		err = Run("two", Provide("i-name", invalid), func() {})
		if assert.Error(t, err) {
			assert.NotContains(t, err.Error(), "two(0)")
			assert.Contains(t, err.Error(), "i-name ")
		}

		err = Run("three", nil, invalid, func() {})
		if assert.Error(t, err) {
			assert.Contains(t, err.Error(), "three(1) ")
		}

		err = Run("four", nil, nil, Cacheable(invalid), func() {})
		if assert.Error(t, err) {
			assert.Contains(t, err.Error(), "four(2) ")
		}
	})
}

func TestVariableImplementsInterfaceLoose(t *testing.T) {
	wrapTest(t, func(t *testing.T) {
		var i2v i2
		var i2vimp i2imp
		i2v = i2vimp
		assert.NoError(t, Run("x",
			Loose(i2v),
			func(i2) {}))
	})
}

func TestVariableImplementsInterfaceExact(t *testing.T) {
	wrapTest(t, func(t *testing.T) {
		var i2v i2
		var i2vimp i2imp
		i2v = i2vimp
		assert.Error(t, Run("x",
			i2v,
			func(i2) {}))
	})
}

func TestInjectorsIncluded(t *testing.T) {
	wrapTest(t, func(t *testing.T) {
		assert.NoError(t, Run("run1",
			s3("s3 value"),
			s0("s0 value"),
			testSeq, func(_ s5, d *Debugging) {
				assert.Equal(t, []string{
					"static static-injector: Debugging [func() *nject.Debugging]",
					"literal literal-value: run1(1) [nject.s0]",
					"static static-injector: TBF(0) [func(nject.s0) nject.s1]",
					"static static-injector: TBF(1) [func(nject.s1) nject.s2]",
					"static static-injector: TBF(3) [func(nject.s2) nject.s5]",
					"invoke invoke-func: run1 invoke func [*func() error]",
					"run fallible-injector: Run()error [func() nject.TerminalError]",
					"final final-func: run1(3) [func(nject.s5, *nject.Debugging)]",
				}, d.Included)
			}))
	})
}

func TestInjectorNamesIncluded(t *testing.T) {
	wrapTest(t, func(t *testing.T) {
		assert.NoError(t, Run("run1",
			s3("s3 value"),
			Provide("S0", s0("s0 value")),
			testSeq, func(_ s5, d *Debugging) {
				assert.Equal(t, []string{
					"Debugging",
					"S0",
					"TBF(0)",
					"TBF(1)",
					"TBF(3)",
					"run1 invoke func",
					"Run()error",
					"run1(3)",
				}, d.NamesIncluded)
			}))
	})
}

func TestInjectorsIncludeExclude(t *testing.T) {
	wrapTest(t, func(t *testing.T) {
		assert.NoError(t, Run("run1",
			s3("s3 value"),
			s0("s0 value"),
			testSeq, func(_ s5, d *Debugging) {
				assert.Equal(t, []string{
					"INCLUDED: static static-injector: Debugging [func() *nject.Debugging] BECAUSE used by final-func: run1(3) [func(nject.s5, *nject.Debugging)] (required)",
					"EXCLUDED: literal literal-value: run1(0) [nject.s3] BECAUSE not used by any remaining providers",
					"INCLUDED: literal literal-value: run1(1) [nject.s0] BECAUSE used by static-injector: TBF(0) [func(nject.s0) nject.s1] (used by static-injector: TBF(1) [func(nject.s1) nject.s2] (used by static-injector: TBF(3) [func(nject.s2) nject.s5] (used by final-func: run1(3) [func(nject.s5, *nject.Debugging)] (required))))",
					"INCLUDED: static static-injector: TBF(0) [func(nject.s0) nject.s1] BECAUSE used by static-injector: TBF(1) [func(nject.s1) nject.s2] (used by static-injector: TBF(3) [func(nject.s2) nject.s5] (used by final-func: run1(3) [func(nject.s5, *nject.Debugging)] (required)))",
					"INCLUDED: static static-injector: TBF(1) [func(nject.s1) nject.s2] BECAUSE used by static-injector: TBF(3) [func(nject.s2) nject.s5] (used by final-func: run1(3) [func(nject.s5, *nject.Debugging)] (required))",
					"EXCLUDED: static static-injector: TBF(2) [func(nject.s3) nject.s4] BECAUSE not used by any remaining providers",
					"INCLUDED: static static-injector: TBF(3) [func(nject.s2) nject.s5] BECAUSE used by final-func: run1(3) [func(nject.s5, *nject.Debugging)] (required)",
					"INCLUDED: invoke invoke-func: run1 invoke func [*func() error] BECAUSE required",
					"INCLUDED: run fallible-injector: Run()error [func() nject.TerminalError] BECAUSE auto-desired (injector with no outputs)",
					"INCLUDED: final final-func: run1(3) [func(nject.s5, *nject.Debugging)] BECAUSE required",
				}, d.IncludeExclude)
			}))
	})
}

func TestInjectorsDebugging(t *testing.T) {
	wrapTest(t, func(t *testing.T) {
		assert.NoError(t, Run("run1",
			s3("s3 value"),
			s0("s0 value"),
			testSeq, func(_ s5, d *Debugging) {
				assert.Greater(t, len(d.Trace), 10000, d.Trace)
			}))
	})
}

func TestInjectorsReproduce(t *testing.T) {
	wrapTest(t, func(t *testing.T) {
		assert.NoError(t, Run("run1",
			s3("s3 value"),
			s0("s0 value"),
			testSeq, func(_ s5, d *Debugging) {
				assert.Regexp(t, ` \*Debugging`, d.Reproduce)
				assert.NotRegexp(t, `// \*nject\.Debugging`, d.Reproduce)
				assert.NotRegexp(t, `\S\t`, d.Reproduce)
				assert.NotRegexp(t, `\S      `, d.Reproduce)
			}))
	})
}
