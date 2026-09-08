package nject

import (
	"fmt"
	"reflect"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type argsWrapper struct {
	t reflect.Type
}

var _ ReflectiveArgs = argsWrapper{}

func (f argsWrapper) NumIn() int             { return f.t.NumIn() }
func (f argsWrapper) In(i int) reflect.Type  { return f.t.In(i) }
func (f argsWrapper) NumOut() int            { return f.t.NumOut() }
func (f argsWrapper) Out(i int) reflect.Type { return f.t.Out(i) }

type wrapShell struct {
	argsWrapper
	inner argsWrapper
	v     reflect.Value
}

var _ ReflectiveWrapper = wrapShell{}

func (w wrapShell) Inner() ReflectiveArgs { return w.inner }
func (w wrapShell) Call(in []reflect.Value) []reflect.Value {
	inner := in[0].Interface().(func([]reflect.Value) []reflect.Value)
	in[0] = reflect.MakeFunc(w.v.Type().In(0), inner)
	return w.v.Call(in)
}

type funcWrapper struct {
	v reflect.Value
}

var _ Reflective = funcWrapper{}

func (f funcWrapper) NumIn() int                              { return f.v.Type().NumIn() }
func (f funcWrapper) In(i int) reflect.Type                   { return f.v.Type().In(i) }
func (f funcWrapper) NumOut() int                             { return f.v.Type().NumOut() }
func (f funcWrapper) Out(i int) reflect.Type                  { return f.v.Type().Out(i) }
func (f funcWrapper) Call(in []reflect.Value) []reflect.Value { return f.v.Call(in) }

type invokeWrapper struct {
	argsWrapper
	v reflect.Value
}

var _ ReflectiveInvoker = invokeWrapper{}

func (f invokeWrapper) Set(imp func([]reflect.Value) []reflect.Value) {
	f.v.Elem().Set(reflect.MakeFunc(f.v.Type().Elem(), imp))
}

func TestManualReflective(t *testing.T) {
	t.Parallel()
	var called bool
	var fCalled bool
	f := func(inner func(), _ s1) {
		fCalled = true
		assert.False(t, called, "before inner")
		inner()
		assert.True(t, called, "after inner")
	}
	w := funcWrapper{v: reflect.ValueOf(f)}
	_ = Run("TestReflective",
		s1("s1"),
		w,
		func() {
			called = true
		},
	)
	assert.True(t, fCalled, "f called")
}

func TestReflective(t *testing.T) {
	t.Parallel()
	var buf string
	type binder func(*Collection, any, any)
	doPrint := func(v ...any) {
		buf += fmt.Sprint(v...)
	}
	cases := []struct {
		name       string
		collection *Collection
		call       func(*testing.T, *Collection, binder)
		call2      func(*testing.T, *Collection, binder)
		call3      func(*testing.T, *Collection, binder)
	}{
		{
			name: "simple",
			collection: Sequence("terminal error",
				int64(8),
				func(i func() error) {
					doPrint("error is", i())
				},
				func(s string) (bool, TerminalError) {
					return strconv.ParseBool(s)
				},
				func(b bool) int {
					doPrint("got", b)
					if b {
						return 1
					}
					return 0
				}),
			call: func(t *testing.T, c *Collection, doBind binder) {
				var x func(string) int
				doBind(c, &x, nil)
				assert.Equal(t, 1, x("true"))
			},
		},
		{
			name: "wrapper",
			collection: Sequence("wrapper",
				func(inner func(string, int) bool, i int) string {
					s := strconv.Itoa(i)
					b := inner(s, i)
					s2 := strconv.FormatBool(b)
					return s2
				},
				func(s string, i int) bool {
					s2 := strconv.Itoa(i)
					return s == s2
				}),
			call: func(t *testing.T, c *Collection, doBind binder) {
				var x func(int) string
				doBind(c, &x, nil)
				assert.Equal(t, "true", x(32))
			},
		},
		{
			name: "with-init",
			collection: Sequence("with-init",
				func(s string, i int) string {
					is := strconv.Itoa(i)
					return s + " " + is
				}),
			call: func(t *testing.T, c *Collection, doBind binder) {
				var x func(string) string
				var y func(int)
				doBind(c, &x, &y)
				y(10)
				y(11)
				assert.Equal(t, "ten 10", x("ten"))
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			names := []string{"regular-bind", "reflective-bind"}
			for i, doBind := range []binder{
				func(c *Collection, invokeF any, initF any) {
					err := c.Bind(invokeF, initF)
					require.NoError(t, err, "bind")
				},
				func(c *Collection, invokeF any, initF any) {
					v := reflect.ValueOf(invokeF)
					t.Log("reflective bind to", v.Type().Elem())
					invokeV := invokeWrapper{
						argsWrapper: argsWrapper{
							t: v.Type().Elem(),
						},
						v: v,
					}
					if initF != nil {
						iv := reflect.ValueOf(initF)
						initV := invokeWrapper{
							argsWrapper: argsWrapper{
								t: iv.Type().Elem(),
							},
							v: iv,
						}
						err := c.Bind(invokeV, initV)
						require.NoError(t, err, "reflective bind")
					} else {
						err := c.Bind(invokeV, nil)
						require.NoError(t, err, "reflective bind")
					}
				},
			} {
				t.Run(names[i], func(t *testing.T) {
					for callNum, call := range []func(*testing.T, *Collection, binder){tc.call, tc.call2, tc.call3} {
						if call == nil {
							continue
						}
						t.Run(fmt.Sprintf("call%d", callNum+1), func(t *testing.T) {
							call(t, tc.collection, doBind)
						})
						rc := rewriteReflective(tc.collection, false)
						t.Run(fmt.Sprintf("call-reflective%d", callNum+1), func(t *testing.T) {
							call(t, rc, doBind)
						})
						rwc := rewriteReflective(tc.collection, true)
						t.Run(fmt.Sprintf("call-reflective-wrapper%d", callNum+1), func(t *testing.T) {
							call(t, rwc, doBind)
						})
					}
				})
			}
		})
	}
}

func rewriteReflective(c *Collection, inner bool) *Collection {
	n := Sequence("redone")
	for _, fm := range c.contents {
		//nolint:govet // fm shadows, yah, duh
		fm := fm.copy()
		switch fm.fn.(type) {
		case ReflectiveWrapper, Reflective:
			//
		default:
			if reflect.TypeOf(fm.fn).Kind() == reflect.Func {
				fm.fn = makeReflectiveShell(fm.fn, inner)
			}
		}
		n.contents = append(n.contents, fm)
	}
	return n
}

func makeReflectiveShell(fn any, inner bool) Reflective {
	v := reflect.ValueOf(fn)
	if !inner {
		return funcWrapper{v: v}
	}
	if v.Type().NumIn() == 0 || v.Type().In(0).Kind() != reflect.Func {
		return funcWrapper{v: v}
	}
	return wrapShell{
		argsWrapper: argsWrapper{t: v.Type()},
		inner:       argsWrapper{t: v.Type().In(0)},
		v:           v,
	}
}
