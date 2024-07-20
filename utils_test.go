package nject

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSaveToInvalid(t *testing.T) {
	f := func() {}
	cases := []struct {
		want  string
		thing interface{}
	}{
		{
			want: "not a valid pointer",
		},
		{
			want:  "is nil",
			thing: (*string)(nil),
		},
		{
			want:  "is not a pointer",
			thing: 7,
		},
		{
			want:  "may not be a pointer to a function",
			thing: &f,
		},
	}
	for _, tc := range cases {
		t.Log(tc.want)
		_, err := SaveTo(tc.thing)
		if assert.Error(t, err, tc.want) {
			assert.Contains(t, err.Error(), tc.want)
		}
	}
}

func TestSaveToValid(t *testing.T) {
	type foo struct {
		i int
	}
	var fooDst foo
	var fooPtrDst *foo
	cases := []struct {
		inject interface{}
		ptr    interface{}
		check  func()
	}{
		{
			inject: foo{i: 7},
			ptr:    &fooDst,
			check: func() {
				assert.Equal(t, foo{i: 7}, fooDst)
			},
		},
		{
			inject: &foo{i: 7},
			ptr:    &fooPtrDst,
			check: func() {
				assert.Equal(t, foo{i: 7}, *fooPtrDst)
			},
		},
	}
	for i, tc := range cases {
		err := Run("x",
			tc.inject,
			MustSaveTo(tc.ptr),
		)
		if assert.NoErrorf(t, err, "%d", i) {
			assert.NotPanics(t, tc.check, "check")
		}
	}
}

func TestCurry(t *testing.T) {
	t.Parallel()
	seq := Sequence("available",
		func() string { return "foo" },
		func() int { return 3 },
		func() uint { return 7 },
	)
	var c1 func(string) string
	var c2 func(bool, bool, string, string) string
	cases := []struct {
		name     string
		fail     string
		curry    interface{}
		check    func(t *testing.T)
		original interface{}
	}{
		{
			curry: &c1,
			original: func(x int, s string) string {
				return fmt.Sprintf("%s-%d", s, x)
			},
			check: func(t *testing.T) {
				assert.Equal(t, "bar-3", c1("bar"))
			},
		},
		{
			curry: &c1,
			original: func(x int, s string, u uint) string {
				return fmt.Sprintf("%s-%d/%d", s, x, u)
			},
			check: func(t *testing.T) {
				assert.Equal(t, "bar-3/7", c1("bar"))
			},
		},
		{
			curry: &c2,
			original: func(b1 bool, x int, b2 bool, s1 string, s2 string, u uint) string {
				return fmt.Sprintf("%v-%d-%v %s %s-%d", b1, x, b2, s1, s2, u)
			},
			check: func(t *testing.T) {
				assert.Equal(t, "true-3-false bee boot-7", c2(true, false, "bee", "boot"))
			},
		},
		{
			curry:    &c2,
			original: func(b1 bool, s1 string, s2 string) string { return "" },
			fail:     "curried function must take fewer arguments",
		},
		{
			curry:    &c2,
			original: func(b1 bool, b2 bool, b3 bool, s1 string, s2 string) string { return "" },
			fail:     "original function takes more arguments of type bool",
		},
		{
			name:  "no original",
			curry: &c2,
			fail:  "original function is not a valid value",
		},
		{
			name:     "no curry",
			original: func(b1 bool, b2 bool, b3 bool, s1 string, s2 string) string { return "" },
			fail:     "curried function is not a valid value",
		},
		{
			name:     "non-pointer",
			curry:    7,
			original: func(b1 bool, b2 bool, b3 bool, s1 string, s2 string) string { return "" },
			fail:     "pointer (to a function)",
		},
		{
			name:     "non-func",
			curry:    seq,
			original: func(b1 bool, b2 bool, b3 bool, s1 string, s2 string) string { return "" },
			fail:     "pointer to a function",
		},
		{
			curry:    &c2,
			original: "original non-func",
			fail:     "first argument to Curry must be a function",
		},
		{
			name:     "nil",
			curry:    (*func())(nil),
			original: func(string) {},
			fail:     "pointer to curried function cannot be nil",
		},
		{
			curry:    &c1,
			original: func(string) {},
			fail:     "same number of outputs",
		},
		{
			curry: &c2,
			original: func(b1 bool, x int, b2 bool, s1 string, s2 string, u uint) int {
				return 22
			},
			fail: "return value #1 has a different type",
		},
		{
			curry: &c1,
			original: func(i1 int, i2 int, s string) string {
				return "foo"
			},
			fail: "cannot curry the same type (int) more than once",
		},
		{
			curry: &c1,
			original: func(uint, int) string {
				return "foo"
			},
			fail: "not all of the string inputs to the curried function were used",
		},
		{
			curry: &c1,
			original: func(s string, inner func(), i int) string {
				return fmt.Sprintf("%s-%d", s, i)
			},
			fail: "may not be a function",
		},
	}

	for _, tc := range cases {
		name := tc.name
		if name == "" {
			name = fmt.Sprintf("%T", tc.original)
		}
		t.Run(name, func(t *testing.T) {
			var called bool
			p, err := Curry(tc.original, tc.curry)
			if tc.fail != "" && err != nil {
				assert.Contains(t, err.Error(), tc.fail, "curry")
				assert.Panics(t, func() {
					_ = MustCurry(tc.original, tc.curry)
				}, "curry")
				return
			} else {
				//nolint:testifylint
				if !assert.NoError(t, err, "curry") {
					return
				}
			}
			err = Run(name, seq, p, func() { called = true })
			if tc.fail != "" {
				if assert.Error(t, err, "run") {
					assert.Contains(t, err.Error(), tc.fail, "run")
				}
				return
			}
			assert.True(t, called, "called")
			tc.check(t)
		})
	}
}
