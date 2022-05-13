package nject

import (
	"reflect"
	"strings"
)

// TODO add ExampleReflective

// Reflective is an alternative provider interface.  Normally, providers are
// are functions or data elements to be injected.  If the provider is a Reflective
// then the methods of Reflective will be called to simulate the Reflective
// being a function.
type Reflective interface {
	ReflectiveArgs
	Call(in []reflect.Value) []reflect.Value
}

// ReflectiveArgs is the part of a Reflective that defines the inputs
// and outputs.
type ReflectiveArgs interface {
	In(i int) reflect.Type
	NumIn() int
	Out(i int) reflect.Type
	NumOut() int
}

// ReflectiveWrapper is a special variant of Reflective where the type of
// the first input is described by Inner().  In(0) will never be called.
// When Call() is invoked, the first argument will be a function that
// takes and returns a slice of reflect.Value -- with the contents of the
// slices determined by Inner()
type ReflectiveWrapper interface {
	Reflective
	Inner() ReflectiveArgs
}

// MakeReflective is a simple wrapper to create a Reflective
func MakeReflective(
	inputs []reflect.Type,
	outputs []reflect.Type,
	function func([]reflect.Value) []reflect.Value,
) Reflective {
	return thinReflective{
		thinReflectiveArgs: thinReflectiveArgs{
			inputs:  inputs,
			outputs: outputs,
		},
		fun: function,
	}
}

type thinReflectiveArgs struct {
	inputs  []reflect.Type
	outputs []reflect.Type
}

var _ ReflectiveArgs = thinReflectiveArgs{}

func (r thinReflectiveArgs) In(i int) reflect.Type  { return r.inputs[i] }
func (r thinReflectiveArgs) NumIn() int             { return len(r.inputs) }
func (r thinReflectiveArgs) Out(i int) reflect.Type { return r.outputs[i] }
func (r thinReflectiveArgs) NumOut() int            { return len(r.outputs) }

type thinReflective struct {
	thinReflectiveArgs
	fun func([]reflect.Value) []reflect.Value
}

var _ Reflective = thinReflective{}

func (r thinReflective) Call(in []reflect.Value) []reflect.Value { return r.fun(in) }

// MakeReflectiveWrapper is a simple wrapper to create a ReflectiveWrapper
//
// The first argument, downIn, is the types that must be recevied in the down
// chain and provided to function.
//
// The second argument, upOut, is the types that are returned on the up chain.
//
// The third argument, downOut, is the types provided in the call to the inner
// function and thus are passed down the down chain.
//
// The forth agument, upIn, is the types returned by the call to the inner
// function and thus are received from the up chain.
//
// When function is called, the first argument will be a reflect.Value, of
// course, that is the value of a function that takes []reflect.Value and
// returns []reflect.Value.
func MakeReflectiveWrapper(
	downIn []reflect.Type,
	upOut []reflect.Type,
	downOut []reflect.Type,
	upIn []reflect.Type,
	function func([]reflect.Value) []reflect.Value,
) ReflectiveWrapper {
	return thinReflectiveWrapper{
		thinReflective: thinReflective{
			thinReflectiveArgs: thinReflectiveArgs{
				inputs:  downIn,
				outputs: upOut,
			},
			fun: function,
		},
		inner: thinReflectiveArgs{
			inputs:  downOut,
			outputs: upIn,
		},
	}
}

type thinReflectiveWrapper struct {
	thinReflective
	inner thinReflectiveArgs
	fun   func([]reflect.Value) []reflect.Value
}

var _ ReflectiveWrapper = thinReflectiveWrapper{}

func (r thinReflectiveWrapper) Inner() ReflectiveArgs { return r.inner }

// wrappedReflective allows Refelective to kinda pretend to be a reflect.Type
type wrappedReflective struct {
	ReflectiveArgs
}

// reflecType is a subset of reflect.Type good enough for use in characterize
type reflectType interface {
	Kind() reflect.Kind
	NumOut() int
	NumIn() int
	In(i int) reflect.Type
	Elem() reflect.Type
	Out(i int) reflect.Type
	String() string
}

var _ reflectType = wrappedReflective{}

func (w wrappedReflective) Kind() reflect.Kind { return reflect.Func }
func (w wrappedReflective) Elem() reflect.Type { panic("call not expected") }

func (w wrappedReflective) String() string {
	in := make([]string, w.NumIn())
	for i := 0; i < w.NumIn(); i++ {
		in[i] = w.In(i).String()
	}
	out := make([]string, w.NumOut())
	for i := 0; i < w.NumOut(); i++ {
		out[i] = w.Out(i).String()
	}
	switch len(out) {
	case 0:
		return "Reflective(" + strings.Join(in, ", ") + ")"
	case 1:
		return "Reflective(" + strings.Join(in, ", ") + ") " + out[0]
	default:
		return "Reflective(" + strings.Join(in, ", ") + ") (" + strings.Join(out, ", ") + ")"
	}
}