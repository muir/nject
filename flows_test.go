package nject

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type flows interface {
	DownFlows() (inputs []reflect.Type, outputs []reflect.Type)
	UpFlows() (consume []reflect.Type, produce []reflect.Type)
	String() string
}

func toTypes(real ...interface{}) []reflect.Type {
	types := make([]reflect.Type, len(real))
	for i, r := range real {
		switch t := r.(type) {
		case reflect.Type:
			types[i] = t
		case typeCode:
			types[i] = t.Type()
		default:
			types[i] = reflect.TypeOf(r)
		}
	}
	return types
}

func flowToStrings(types []typeCode) []string {
	types = noNoType(types)
	s := make([]string, len(types))
	for i, t := range types {
		s[i] = t.Type().String()
	}
	return s
}

func toStrings(types []reflect.Type) []string {
	s := make([]string, len(types))
	for i, t := range types {
		s[i] = t.String()
	}
	return s
}

func TestFlows(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		provider interface{}
		upIn     []interface{}
		upOut    []interface{}
		downIn   []interface{}
		downOut  []interface{}
		class    classType
	}{
		{
			name:     "fallable injector",
			provider: func(int, string) TerminalError { return nil },
			upOut:    []interface{}{errorType},
			downIn:   []interface{}{3, ""},
		},
		{
			name:     "injecting error",
			provider: func(int, string) error { return nil },
			downOut:  []interface{}{errorType},
			downIn:   []interface{}{3, ""},
		},
		{
			name:     "wrapper",
			provider: func(func(int, string) bool, float64) float32 { return 3.2 },
			downIn:   []interface{}{float64(3.3)},
			downOut:  []interface{}{3, ""},
			upOut:    []interface{}{float32(9.2)},
			upIn:     []interface{}{true},
		},
		{
			name:     "constant",
			provider: int64(32),
			downOut:  []interface{}{int64(10)},
		},
		{
			name: "collection",
			provider: Sequence("x",
				int64(38), // int64 down/out
				func(string, float32) bool { return true },                    // string, float32 down/in; bool down/out
				func(string, bool) {},                                         // string, bool down/in
				func(bool) TerminalError { return nil },                       // bool down/in; error up/out
				func(func(float64) string, bool) complex128 { return 7 + 2i }, // bool down/in; float64 down/out; complex128 up/out; string up/in
			),
			downIn:  []interface{}{"", float32(3)},
			downOut: []interface{}{true, int64(10), float64(10)},
			upOut:   []interface{}{errorType, complex128(7 + 2i)},
			upIn:    []interface{}{""},
		},
		{
			name: "reflective injector",
			provider: MakeReflective(
				toTypes(9, ""),
				toTypes(float32(8)),
				func([]reflect.Value) []reflect.Value {
					return nil
				}),
			downIn:  []interface{}{9, ""},
			downOut: []interface{}{float32(8)},
		},
		{
			name: "Reflective wrapper",
			provider: MakeReflective(
				toTypes(func(string) bool { return true }, 9, ""),
				toTypes(float32(8)),
				func([]reflect.Value) []reflect.Value {
					return nil
				}),
			downIn:  []interface{}{9, ""},
			upOut:   []interface{}{float32(8)},
			upIn:    []interface{}{true},
			downOut: []interface{}{""},
			class:   wrapperFunc,
		},
		{
			name: "ReflectiveWrapper",
			provider: MakeReflectiveWrapper(
				toTypes(5, 8, ""),
				toTypes(float32(7)),
				toTypes(""),
				toTypes(false),
				func([]reflect.Value) []reflect.Value {
					return nil
				}),
			downIn:  []interface{}{7, 9, ""},
			upOut:   []interface{}{float32(8)},
			downOut: []interface{}{""},
			upIn:    []interface{}{true},
			class:   wrapperFunc,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f, ok := tc.provider.(flows)
			if ok {
				t.Log("->", f.String())
			} else {
				t.Log("->", reflect.TypeOf(tc.provider))
				f = Provide(tc.name, tc.provider)
			}

			wantDownIn, wantDownOut := toTypes(tc.downIn...), toTypes(tc.downOut...)
			wantUpIn, wantUpOut := toTypes(tc.upIn...), toTypes(tc.upOut...)

			fCheck := func(f flows, context string) {
				t.Log(context)
				downIn, downOut := f.DownFlows()
				upIn, upOut := f.UpFlows()
				t.Log("down/in", downIn)
				t.Log("down/out", downOut)
				t.Log("up/in", upIn)
				t.Log("up/out", upOut)
				assert.ElementsMatchf(t, toStrings(wantDownIn), toStrings(downIn), "down in %s", context)
				assert.ElementsMatchf(t, toStrings(wantDownOut), toStrings(downOut), "down out %s", context)
				assert.ElementsMatchf(t, toStrings(wantUpIn), toStrings(upIn), "up in %s", context)
				assert.ElementsMatchf(t, toStrings(wantUpOut), toStrings(upOut), "up out %s", context)
			}

			fCheck(f, "direct")

			charCheck := func(p *provider) {
				t.Log("checking against characterize flows too")
				fm, err := handlerRegistry.characterizeFuncDetails(p, charContext{})
				if err != nil {
					var e2 error
					fm, e2 = handlerRegistry.characterizeFuncDetails(p, charContext{inputsAreStatic: true})
					require.NoErrorf(t, e2, "static characterize, and non-static: %s", err)
				}
				if tc.class != unsetClassType {
					require.Equalf(t, tc.class, fm.class, "class type %s vs %s", tc.class, fm.class)
				}
				assert.Equal(t, toStrings(wantUpOut), flowToStrings(fm.flows[returnParams]), "char up out")
				assert.Equal(t, toStrings(wantDownOut), flowToStrings(fm.flows[outputParams]), "char down out")
				assert.Equal(t, toStrings(wantDownIn), flowToStrings(fm.flows[inputParams]), "char down in")
				assert.Equal(t, toStrings(wantUpIn), flowToStrings(fm.flows[receivedParams]), "char up in")

				fCheck(p, "characterized")
			}

			switch p := f.(type) {
			case *provider:
				charCheck(p)
			case *Collection:
				// skip
			default:
				assert.Failf(t, "unexpected type", "unexpected type %T", f)
			}
		})
	}
}
