package nject

import (
	"reflect"
	"strings"
)

// TODO add ExampleReflective

// ReflectiveInvoker is an alternative provider interface that can be used
// for invoke and initialize functions.  The key for those functions is that
// their implmentation is provided by Collection.Bind.
type ReflectiveInvoker interface {
	ReflectiveArgs
	Set(func([]reflect.Value) []reflect.Value)
}

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
// the first input is described by Inner().  In(0) must return the
// type of func([]reflect.Type) []reflect.Type.
//
// When Call() is invoked, In(0) must be as described by Inner().
type ReflectiveWrapper interface {
	Reflective
	Inner() ReflectiveArgs
}

// MakeReflective is a simple utility to create a Reflective
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

type reflectiveBinder struct {
	thinReflective
}

var _ ReflectiveInvoker = &reflectiveBinder{}
var _ Reflective = reflectiveBinder{}

func (b *reflectiveBinder) Set(fun func([]reflect.Value) []reflect.Value) {
	b.fun = fun
}

func (r thinReflective) Call(in []reflect.Value) []reflect.Value { return r.fun(in) }

// MakeReflectiveWrapper is a utility to create a ReflectiveWrapper
//
// The first argument, downIn, is the types that must be recevied in the down
// chain and provided to function.  This does not include the
// func([]reflec.Value) []reflect.Value that is actually used for the first
// argument.
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
//
// EXPERIMENTAL: this is currently considered experimental and could be removed
// in a future release.  If you're using this, please open a pull request to
// remove this comment.
func MakeReflectiveWrapper(
	downIn []reflect.Type,
	upOut []reflect.Type,
	downOut []reflect.Type,
	upIn []reflect.Type,
	function func([]reflect.Value) []reflect.Value,
) ReflectiveWrapper {
	modifiedDownIn := make([]reflect.Type, len(downIn)+1)
	modifiedDownIn[0] = reflectiveFuncType
	for i, t := range downIn {
		modifiedDownIn[i+1] = t
	}
	return thinReflectiveWrapper{
		thinReflective: thinReflective{
			thinReflectiveArgs: thinReflectiveArgs{
				inputs:  modifiedDownIn,
				outputs: upOut,
			},
			fun: function,
		},
		inner: thinReflectiveArgs{
			inputs:  downOut,
			outputs: upIn,
		},
		fun: function,
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
