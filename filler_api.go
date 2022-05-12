package nject

import (
	"reflect"
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

// PostActionFuncArg are functional arguments to PostActionByTag,
// PostActionByName, and PostActionByType.
type PostActionFuncArg func(*postActionOption)

type postActionOption struct {
	function         interface{}
	fill             bool
	fillSet          bool
	matchToInterface bool
}

// generatedFromInjectionChain is a special kind of provider that inspects the rest of the
// injection chain to replace itself with a regular provider.  The ReplaceSelf method will
// be called only once.
type generatedFromInjectionChain interface {
	String() string
	ReplaceSelf(chainBefore Collection, chainAfter Collection) (selfReplacement Provider, err error)
}

var _ generatedFromInjectionChain = gfic{}

type gfic struct {
	name string
	f    func(chainBefore Collection, chainAfter Collection) (selfReplacement Provider, err error)
}

func (g gfic) String() string { return g.name }

func (g gfic) ReplaceSelf(before Collection, after Collection) (selfReplacement Provider, err error) {
	return g.f(before, after)
}

// GenerateFromInjectionChain creates a very special provider from a function
// that examines the injection chain and then returns a replacement provider.
// The first parameter for the function is a Collection representing all the providers
// that are earlier in the chain from from the new special provider; the second
// parameter is a Collection representing all the providers that are later
// in the chain from the new special provider.
func GenerateFromInjectionChain(
	name string,
	f func(chainBefore Collection, chainAfter Collection) (selfReplacement Provider, err error),
) generatedFromInjectionChain {
	return gfic{
		name: name,
		f:    f,
	}
}

type ignore struct{}

// FillerFuncArg is a functional argument for MakeStructBuilder
type FillerFuncArg func(*fillerOptions)

// WithTag sets the struct tag to use for per-struct-field
// directives in MakeStructBuilder.  The default tag is "nject"
//
// EXPERIMENTAL: this is currently considered experimental
// and could be removed in a future release.  If you are using
// this, please open a pull request to remove this comment.
func WithTag(tag string) FillerFuncArg {
	return func(o *fillerOptions) {
		o.tag = tag
	}
}

// TODO: WithOptionalMethod

// WithMethodCall looks up a method on the struct being
// filled or built and adds a method invocation to the
// dependency chain.  The method can be any kind of function
// provider (the last function, a wrapper, etc).  If there
// is no method of that name, then MakeStructBuilder will
// return an error.
//
// EXPERIMENTAL: this is currently considered experimental
// and could be removed in a future release.  If you are using
// this, please open a pull request to remove this comment.
func WithMethodCall(methodName string) FillerFuncArg {
	// Implementation note:
	// We'll use a Reflective to invoke the method using the
	// the version of the method that takes an explicit
	// receiver.
	return func(o *fillerOptions) {
		o.postMethodName = append(o.postMethodName, methodName)
	}
}

// PostActionByTag establishes a tag value that indicates that
// after the struct is built or filled, a function should be called
// passing a pointer to the tagged field to the function.  The
// function must take as an input parameter a pointer to the type
// of the field or it must take as an input paraemter an interface
// type that the field implements.  interface{} is allowed.
// This function will be added to the injection chain after the
// function that builds or fills the struct.  If there is also a
// WithMethodCall, this function will run before that.
//
// EXPERIMENTAL: this is currently considered experimental
// and could be removed in a future release.  If you are using
// this, please open a pull request to remove this comment.
func PostActionByTag(tagValue string, function interface{}, opts ...PostActionFuncArg) FillerFuncArg {
	// Implementation note:
	// There could be more than one field using the same type so
	// the normal chain parameter passing methods won't work.
	// To get around that, we'll create a Reflective that is a
	// thin wrapper around a function.  We'll select the closest
	// match between the function input and the field and replace
	// that type in the list of inputs with the struct being filled.
	// The actuall Call() we will grab the field from the struct
	// using it's index and use that to call the function.
	options := makePostActionOption(function, opts)
	return func(o *fillerOptions) {
		o.postActionByTag[tagValue] = options
	}
}

// PostActionByType arranges to call a function for every field in
// struct that is being filled where the type of the field in
// the struct exactly matches the first input parameter of the
// provided function.  PostActionByType calls are made after
// PostActionByTag calls, but before WithMethodCall invocations.
//
// If there is no match to the type of the function, then the function
// is not invoked.
//
// EXPERIMENTAL: this is currently considered experimental
// and could be removed in a future release.  If you are using
// this, please open a pull request to remove this comment.
func PostActionByType(function interface{}, opts ...PostActionFuncArg) FillerFuncArg {
	options := makePostActionOption(function, opts)
	return func(o *fillerOptions) {
		o.postActionByType = append(o.postActionByType, options)
	}
}

// PostActionByName arranges to call a function passing in the field that
// has a matching name.  PostActionByName happens before PostActionByType
// and after PostActionByTag calls.
//
// EXPERIMENTAL: this is currently considered experimental
// and could be removed in a future release.  If you are using
// this, please open a pull request to remove this comment.
func PostActionByName(name string, function interface{}, opts ...PostActionFuncArg) FillerFuncArg {
	options := makePostActionOption(function, opts)
	return func(o *fillerOptions) {
		o.postActionByName[name] = options
	}
}

// FillExisting changes the behavior of MakeStructBuilder so that it
// fills fields in a struct that it receives from upstream in the
// provider chain rather than starting fresh with a new structure.
//
// EXPERIMENTAL: this is currently considered experimental
// and could be removed in a future release.  If you are using
// this, please open a pull request to remove this comment.
func FillExisting(o *fillerOptions) {
	o.create = false
}

func makePostActionOption(function interface{}, userOpts []PostActionFuncArg, typeOpts ...PostActionFuncArg) postActionOption {
	options := postActionOption{
		function: function,
	}
	for _, opt := range typeOpts {
		opt(&options)
	}
	for _, opt := range userOpts {
		opt(&options)
	}
	return options
}

// TODO: add ExampleWithFIll

// WithFill overrides the default behaviors of PostActionByType, PostActionByName,
// and PostActionByTag with respect to the field being automatically filled.
// By default, if there is a post-action that that recevies a pointer to the
// field, then the field will not be filled from the injection chain.
//
// EXPERIMENTAL: this is currently considered experimental
// and could be removed in a future release.  If you are using
// this, please open a pull request to remove this comment.
func WithFill(b bool) PostActionFuncArg {
	return func(o *postActionOption) {
		o.fill = b
		o.fillSet = true
	}
}

// MatchToOpenInterface requires that the post action function have exactly one
// open interface type (interface{}) in its arguments list.  A pointer to the
// field will be passed to the interface parameter.
//
// EXPERIMENTAL: this is currently considered experimental
// and could be removed in a future release.  If you are using
// this, please open a pull request to remove this comment.
func MatchToOpenInterface(b bool) PostActionFuncArg {
	return func(o *postActionOption) {
		o.matchToInterface = true
	}
}

// MustMakeStructBuilder wraps a panic around failed
// MakeStructBuilder calls
//
// EXPERIMENTAL: this is currently considered experimental
// and could be removed in a future release.  If you are using
// this, please open a pull request to remove this comment.
func MustMakeStructBuilder(model interface{}, opts ...FillerFuncArg) Provider {
	p, err := MakeStructBuilder(model, opts...)
	if err != nil {
		panic(err.Error())
	}
	return p
}

// TODO: add ExampleProvideRequireGap

// ProvideRequireGap identifies types that are required but are not provided.
func ProvideRequireGap(provided []reflect.Type, required []reflect.Type) []reflect.Type {
	have := make(map[typeCode]struct{})
	for _, t := range provided {
		have[getTypeCode(t)] = struct{}{}
	}
	var missing []reflect.Type
	for _, t := range required {
		if _, ok := have[getTypeCode(t)]; ok {
			continue
		}
		missing = append(missing, t)
	}
	return missing
}
