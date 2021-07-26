package nject

// TODO: is CallsInner() useful?  Remove if not.

import (
	"fmt"
	"reflect"
	"sync/atomic"
)

// Collection holds a sequence of providers and sub-collections
type Collection struct {
	name     string
	contents []*provider
}

// Provider is an individual injector (function, constant, or
// wrapper).  Functions that take injectors, take interface{}.
// Functions that return invjectors return Provider so that
// methods can be attached.
type Provider interface {
	thing
}

// Sequence creates a Collection of providers.  Each collection must
// have a name.  The providers can be: functions, variables, or
// *Collections, or *Providers.
//
// Functions must match one of the expected patterns.
//
// Injectors specified here will be separated into two sets:
// ones that are run once per bound chain (STATIC); and
// ones that are run for each invocation (RUN).  Memoized
// functions are in the STATIC chain but they only get run once
// per input combination.  Literal values are inserted before
// the STATIC chain starts.
//
// Each set will run in the order they were given here.  Providers
// whose output is not consumed are skipped unless they are marked
// with Required.  Providers that produce no output are always run.
//
// Previsously created *Collection objects are considered providers along
// with *Provider, named functions, anoymous functions, and literal values.
func Sequence(name string, providers ...interface{}) *Collection {
	return newCollection(name, providers...)
}

var clusterId int32 = 1

// Cluster is a variation on Sequence() with the additional behavior
// that all of the providers in the in the cluster will be included
// or excluded as a group.  This doesn't apply to providers that cannot
// be included at all.  It also downgrades providers that are in the
// cluster that would normally be considered desired because they don't
// return anything and aren't wrappers: they're no longer considered
// desired.
func Cluster(name string, providers ...interface{}) *Collection {
	c := newCollection(name, providers...)
	id := atomic.AddInt32(&clusterId, 1)
	for _, fm := range c.contents {
		fm.cluster = id
	}
	return c
}

// Append appends additional providers onto an existing collection
// to create a new collection.  The additional providers may be
// value literals, functions, Providers, or *Collections.
func (c *Collection) Append(name string, funcs ...interface{}) *Collection {
	nc := newCollection(name, funcs...)
	nc.contents = append(c.contents, nc.contents...)
	return nc
}

// Provide wraps an individual provider.  It allows the provider
// to be named.  The return value is chainable with with annotations
// like Cacheable() and Required().  It can be included in a collection.
// When providers are not named, they get their name from their position
// in their collection combined with the name of the collection they are in.
//
// When used on an existing Provider, it creates an annotated copy of that provider.
func Provide(name string, fn interface{}) Provider {
	return newThing(fn).modify(func(fm *provider) {
		fm.origin = name
	})
}

// MustCache creates an Inject item and annotates it as required to be
// in the STATIC set.  If it cannot be placed in the STATIC set
// then any collection that includes it is invalid.
func MustCache(fn interface{}) Provider {
	return newThing(fn).modify(func(fm *provider) {
		fm.mustCache = true
		fm.cacheable = true
	})
}

// Cacheable creates an inject item and annotates it as allowed to be
// in the STATIC chain.  Without this annotation, MustCache, or
// Memoize, a provider will be in the RUN chain.
//
// When used on an existing Provider, it creates an annotated copy of that provider.
func Cacheable(fn interface{}) Provider {
	return newThing(fn).modify(func(fm *provider) {
		fm.cacheable = true
	})
}

// Singleton marks a provider as a forced singleton.  The provider will be invoked
// only once even if it is included in multiple different Sequences.  It will be in
// the the STATIC chain.  There is no check that the input arguments available at the
// time the provider would be called are consistent from one invocation to the next.
// The provider will be called exactly once with whatever inputs are provided the
// in the first chain that invokes the provider.
//
// An alternative way to get singleton behavior is with Memoize() combined with
// MustChace().
func Singleton(fn interface{}) Provider {
	return newThing(fn).modify(func(fm *provider) {
		fm.singleton = true
		fm.mustCache = true
		fm.cacheable = true
	})
}

// NotCacheable creates an inject item and annotates it as not allowed to be
// in the STATIC chain.  With this annotation, Cacheable is ignored and MustCache
// causes an invalid chain.
//
// When used on an existing Provider, it creates an annotated copy of that provider.
func NotCacheable(fn interface{}) Provider {
	return newThing(fn).modify(func(fm *provider) {
		fm.notCacheable = true
	})
}

// Memoize creates a new InjectItem that is tagged as Cacheable
// further annotated so that it only executes once per
// input parameter values combination.  This cache is global among
// all Sequences.  Memoize can only be used on functions
// whose inputs are valid map keys (interfaces, arrays
// (not slices), structs, pointers, and primitive types).  It is
// further restrict that it cannot handle private (not exported)
// fields inside structs.
//
// Memoize only works in the STATIC provider set.
//
// Combine Memoize with MustCache to make sure that Memoize can actually
// function as expected.
//
// When used on an existing Provider, it creates an annotated copy of that provider.
//
// As long as consistent injection chains are used Memoize + MustCache can
// gurantee singletons.
func Memoize(fn interface{}) Provider {
	return newThing(fn).modify(func(fm *provider) {
		fm.memoize = true
		fm.cacheable = true
	})
}

// Required creates a new provider and annotates it as
// required: it will be included in the provider chain even
// if its outputs are not used.
//
// When used on an existing Provider, it creates an annotated copy of that provider.
func Required(fn interface{}) Provider {
	return newThing(fn).modify(func(fm *provider) {
		fm.required = true
	})
}

// Desired creates a new provider and annotates it as
// desired: it will be included in the provider chain
// unless doing so creates an un-met dependency.
//
// Injectors and wrappers that have no outputs are automatically
// considered desired.
//
// When used on an existing Provider, it creates an annotated copy of that provider.
func Desired(fn interface{}) Provider {
	return newThing(fn).modify(func(fm *provider) {
		fm.desired = true
	})
}

// MustConsume creates a new provider and annotates it as
// needing to have all of its output values consumed.  If
// any of its output values cannot be consumed then the
// provider will be excluded from the chain even if that
// renders the chain invalid.
//
// A that is received by a provider and then provided by
// that same provider is not considered to have been consumed
// by that provider.
//
// For example:
//
//	// All outputs of A must be consumed
//	Provide("A", MustConsume(func() string) { return "" } ),
//
//	// Since B takes a string and provides a string it
//	// does not count as suming the string that A provided.
//	Provide("B", func(string) string { return "" }),
//
//	// Since C takes a string but does not provide one, it
//	// counts as consuming the string that A provided.
//	Provide("C", func(string) int { return 0 }),
//
// MustConsume works only in the downward direction of the
// provider chain.  In the upward direction (return values)
// all values must be consumed.
//
// When used on an existing Provider, it creates an annotated copy of that provider.
func MustConsume(fn interface{}) Provider {
	return newThing(fn).modify(func(fm *provider) {
		fm.mustConsume = true
	})
}

// ConsumptionOptional creates a new provider and annotates it as
// allowed to have some of its return values ignored.
// Without this annotation, a wrap function will not be included
// if some of its return values are not consumed.
//
// In the downward direction, optional consumption is the default.
//
// When used on an existing Provider, it creates an annotated copy of that provider.
func ConsumptionOptional(fn interface{}) Provider {
	return newThing(fn).modify(func(fm *provider) {
		fm.consumptionOptional = true
	})
}

// Is this useful?   Don't export for now.
//
// CallsInner annotates a wrap function to promise that it
// will always invoke its inner() function.  Without this
// annotation, wrap functions that come before other wrap
// functions or the final function that return values that
// do not have a zero value that can be created with reflect
// are invalid.
//
// When used on an existing Provider, it creates an annotated copy of that provider.
func callsInner(fn interface{}) Provider {
	return newThing(fn).modify(func(fm *provider) {
		fm.callsInner = true
	})
}

// Loose annotates a wrap function to indicate that when trying
// to match types against the outputs and return values from this
// provider, an in-exact match is acceptable.  This matters when inputs and
// returned values are specified as interfaces.  With the Loose
// annotation, an interface can be matched to the outputs and/or
// return values of this provider if the output/return value
// implements the interface.
//
// By default, an exact match of types is required for all providers.
func Loose(fn interface{}) Provider {
	return newThing(fn).modify(func(fm *provider) {
		fm.loose = true
	})
}

// NonFinal annotates a provider to say that it shouldn't be considered the
// final provider in a list of providers.  This is to make it possible to
// insert a provider into a list of providers late in the chain without
// actually being the final provider.  It's easy to insert a final at the
// start of the chain -- you simply list it first.  It's easy to insert a
// final provider.  Without NonFinal, it's hard or impossible to insert
// a provider very late in the chain.  If NonFinal providers are invoked,
// they will be called before the final provider.
func NonFinal(fn interface{}) Provider {
	return newThing(fn).modify(func(fm *provider) {
		fm.nonFinal = true
	})
}

// Bind expects to receive two function pointers for functions
// that are not yet defined.  Bind defines the functions.  The
// first function is called to invoke the Collection of providers.
//
// The inputs to the invoke function are passed into the provider
// chain.  The value returned from the invoke function comes from
// the values returned by the provider chain (from middleware and
// from the final func).
//
// The second function is optional.  It is called to initialize the
// provider chain.  Once initialized, any further calls to the initialize
// function are ignored.
//
// The inputs to the initialization function are injected into the
// head of the provider chain.  The static portion of the provider
// chain will run once.  The values returned from the initialization
// function come from the values available after the static portion
// of the provider chain runs.
//
// Bind pre-computes as much as possible so that the invokeFunc is fast.
//
// Each call to Bind() with unique providers may leak a small amount of memory,
// creating durable type maps and closures to handle memoization and singletons.
// Calls to the invokeFunc do not leak memory except where there are new inputs to
// providers marked Memoize().
func (c *Collection) Bind(invokeFunc interface{}, initFunc interface{}) error {
	if err := c.bindFast(invokeFunc, initFunc); err != nil {
		invokeF := newProvider(invokeFunc, -1, c.name+" invoke func")
		var initF *provider
		if initFunc != nil {
			initF = newProvider(initFunc, -1, c.name+" initialization func")
		}

		debugOutput := captureDoBindDebugging(c, invokeF, initF)
		return &njectError{
			err:     err,
			details: debugOutput,
		}
	}
	return nil
}

func (c *Collection) bindFast(invokeFunc interface{}, initFunc interface{}) error {
	invokeF := newProvider(invokeFunc, -1, c.name+" invoke func")
	var initF *provider
	if initFunc != nil {
		initF = newProvider(initFunc, -1, c.name+" initialization func")
	}

	debugLock.RLock()
	defer debugLock.RUnlock()
	return doBind(c, invokeF, initF, true)
}

// TODO: add an example

// SetCallback expects to receive a function as an argument.  SetCallback() will call
// that function.  That function in turn should take one or two functions
// as arguments.  The first argument must be an invoke function (see Bind).
// The second argument (if present) must be an init function.  The invoke func
// (and the init func if present) will be created by SetCallback() and passed
// to the function SetCallback calls.
func (c *Collection) SetCallback(setCallbackFunc interface{}) error {
	setter := reflect.ValueOf(setCallbackFunc)
	setterType := setter.Type()
	if setterType.Kind() != reflect.Func {
		return fmt.Errorf("SetCallback must be passed a function")
	}
	if setterType.NumIn() < 1 || setterType.NumIn() > 2 {
		return fmt.Errorf("SetCallback function argument must take one or two inputs")
	}
	if setterType.NumOut() > 0 {
		return fmt.Errorf("SetCallback function argument must return nothing")
	}
	for i := 0; i < setterType.NumIn(); i++ {
		if setterType.In(i).Kind() != reflect.Func {
			return fmt.Errorf("SetCallback function argument #%d must be a function", i+1)
		}
	}
	var err error
	invokePtr := reflect.New(setterType.In(0)).Interface()
	if setterType.NumIn() == 2 {
		initPtr := reflect.New(setterType.In(1)).Interface()
		err = c.Bind(invokePtr, initPtr)
		if err == nil {
			setter.Call([]reflect.Value{
				reflect.ValueOf(invokePtr).Elem(),
				reflect.ValueOf(initPtr).Elem()})
		}
	} else {
		err = c.Bind(invokePtr, nil)
		if err == nil {
			setter.Call([]reflect.Value{reflect.ValueOf(invokePtr).Elem()})
		}
	}
	return err
}

// Run is different from bind: the provider chain is run, not bound
// to functions.
//
// The only return value from the final function that is captured by Run()
// is error.  Run will return that error value.  If the final function does
// not return error, then run will return nil if it was able to execute the
// collection and function.  Run can return error because the final function
// returned error or because the provider chain was not valid.
//
// Nothing is pre-computed with Run(): the run-time cost from nject is higher
// than calling an invoke function defined by Bind().
//
// Predefined Collection objects are considered providers along with InjectItems,
// functions, and literal values.
//
// Each call to Run() with unique providers may leak a small amount of memory,
// creating durable type maps and closures to handle memoization and singletons.
func Run(name string, providers ...interface{}) error {
	c := Sequence(name,
		// include a default error responder so that the
		// error return from Run() does not pull in any
		// providers.
		Provide("Run()error", func() TerminalError {
			return nil
		})).
		Append(name, providers...)
	var invoke func() error
	err := c.Bind(&invoke, nil)
	if err != nil {
		return err
	}
	return invoke()
}

// MustRun is a wrapper for Run().  It panic()s if Run() returns error.
func MustRun(name string, providers ...interface{}) {
	err := Run(name, providers...)
	if err != nil {
		panic(err)
	}
}

// MustBindSimple binds a collection with an invoke function that takes no
// arguments and returns no arguments.  It panic()s if Bind() returns error.
func MustBindSimple(c *Collection, name string) func() {
	var invoke func()
	MustBind(c, &invoke, nil)
	return invoke
}

// MustBindSimpleError binds a collection with an invoke function that takes no
// arguments and returns error.
func MustBindSimpleError(c *Collection, name string) func() error {
	var invoke func() error
	MustBind(c, &invoke, nil)
	return invoke
}

// MustBind is a wrapper for Collection.Bind().  It panic()s if Bind() returns error.
func MustBind(c *Collection, invokeFunc interface{}, initFunc interface{}) {
	err := c.Bind(invokeFunc, initFunc)
	if err != nil {
		panic(DetailedError(err))
	}
}

// MustSetCallback is a wrapper for Collection.SetCallback().  It panic()s if SetCallback() returns error.
func MustSetCallback(c *Collection, binderFunction interface{}) {
	err := c.SetCallback(binderFunction)
	if err != nil {
		panic(DetailedError(err))
	}
}

// Reflective is an alternative provider interface.  Normally, providers are
// are functions or data elements to be injected.  If the provider is a Reflective
// then the methods of Reflective will be called to simulate the Reflective
// being a function.
type Reflective interface {
	In(i int) reflect.Type
	NumIn() int
	Out(i int) reflect.Type
	NumOut() int
	Call(in []reflect.Value) []reflect.Value
}

type ignore struct{}

// FillerFuncArg is a functional argument for
// MakeStructBuilder and MakeStructFiller.
type FillerFuncArg func(*fillerOptions)

// WithTag sets the struct tag to use for per-struct-field
// directives in MakeStructBuilder and MakeStructFiller.  The
// default tag is "nject"
func WithTag(tag string) FillerFuncArg {
	return func(o *fillerOptions) {
		o.tag = tag
	}
}

// WithPostMethod looks up a method on the struct being
// filled or built and adds a method invocation to the
// dependency chain.  The method can be any kind of function
// provider (the last function, a wrapper, etc).  If there
// is no method of that name, then MakeStructBuilder or
// MakeStructFiller will return error.
func WithPostMethod(methodName string) FillerFuncArg {
	// Implementation note:
	// We'll use a Reflective to invoke the method using the
	// the version of the method that takes an explicit
	// receiver.
	return func(o *fillerOptions) {
		o.postMethodName = methodName
	}
}

// WithFieldFiller establishes a tag value that indicates that
// after the struct is built or filled, a function should be called
// passing a pointer to the tagged field to the function.  The
// function must take as an input parameter a pointer to the type
// of the field or it must take as an input paraemter an interface
// type that the field implements.  interface{} is allowed.
// This function will be added to the injection chain after the
// function that builds or fills the struct.  If there is also a
// PostMethod, this function will run before that.
func WithFieldFiller(tagValue string, function interface{}) FillerFuncArg {
	// Implementation note:
	// There could be more than one field using the same type so
	// the normal chain parameter passing methods won't work.
	// To get around that, we'll create a Reflective that is a
	// thin wrapper around a function.  We'll select the closest
	// match between the function input and the field and replace
	// that type in the list of inputs with the struct being filled.
	// The actuall Call() we will grab the field from the struct
	// using it's index and use that to call the function.
	return func(o *fillerOptions) {
		o.fieldFiller[tagValue] = function
	}
}

// MakeStructBuilder generates a Provider that wants to receive as
// arguments all of the fields of the struct and returns the struct
// as what it provides.
//
// The input model must be a struct: if not MakeStructFiller
// will panic.  Model may be a pointer to a struct or a struct.
// Unexported fields are always ignored.
// Passing something other than a struct or pointer to a struct to
// MakeStructBuilder results is an error. Unknown tag values is an error.
//
// Struct tags can be used to control the
// behavior: the argument controls the name of the struct tag used.
// A struct tag of "-" or "ignore" indicates that the field should not
// be filled.  A tag of "fill" is accepted but doesn't do anything as it's
// the default.
//
// Embedded structs can either be filled as a whole or they can be
// filled field-by-field.  Tag with "whole" or "blob" to fill the embedded
// struct all at once.  Tag with "fields" to fill the fields of the
// embedded struct individually.  The default is "fields".
func MakeStructBuilder(model interface{}, opts ...FillerFuncArg) (Provider, error) {
	filler, needIgnore, err := makeFiller(model, true, opts)
	if err != nil {
		return nil, err
	}
	if needIgnore {
		return Sequence(fmt.Sprintf("builder seq for %T", model),
			ignore{},
			Provide(fmt.Sprintf("builder for %T", model), filler)), nil
	}
	return Provide(fmt.Sprintf("builder for %T", model), filler), nil
}

// MustMakeStructBuilder wraps a panic around failed
// MakeStructBuilder calls
func MustMakeStructBuilder(model interface{}, opts ...FillerFuncArg) Provider {
	p, err := MakeStructBuilder(model, opts...)
	if err != nil {
		panic(err.Error())
	}
	return p
}

// MakeFuncFiller is like MakeFuncBuilder except that the generated
// function takes as input a pointer to the model that
// needs to be filled out rather than creating a new model.
// Passing something other than a pointer to a struct to MakeStructFiller
// results in an immediate panic.
func MakeStructFiller(model interface{}, opts ...FillerFuncArg) (Provider, error) {
	filler, _, err := makeFiller(model, false, opts)
	if err != nil {
		return nil, err
	}
	return Provide(fmt.Sprintf("filler for %T", model), filler), nil
}

// MustMakeStructFiller wraps a panic around failed
// MakeStructFiller calls
func MustMakeStructFiller(model interface{}, opts ...FillerFuncArg) Provider {
	p, err := MakeStructFiller(model, opts...)
	if err != nil {
		panic(err.Error())
	}
	return p
}
