package nject

// TODO: is CallsInner() useful?  Remove if not.

import (
	"fmt"
	"reflect"
	"sync/atomic"
)

// Collection holds a sequence of providers and sub-collections.  A Collection
// implements the Provider interface and can be used anywhere a Provider is
// required.
type Collection struct {
	// The above comment is wrong but helps understanding as-is.
	// A collection holds a list of *provider not Provider. That list is already flattened.
	name     string
	contents []*provider
}

var _ Provider = &Collection{}

// Provider is an individual injector (function, constant, or
// wrapper).  Functions that take injectors, take any.
// Functions that return invjectors return Provider so that
// methods can be attached.
type Provider interface {
	thing
	String() string

	// For single providers, DownFlows includes all inputs and
	// all outputs.  For collections, Downflows only includes
	// the net inputs and net outputs.
	DownFlows() (inputs []reflect.Type, outputs []reflect.Type)

	// For single providers, Upflows includes all consumes and
	// all returns.  For collections, Upflows only includes
	// the net consumes and returns.
	//
	// Providers that return TerminalError are a special case and count as
	// producing error.
	UpFlows() (consume []reflect.Type, produce []reflect.Type)
}

// Sequence creates a Collection of providers.  Each collection must
// have a name.  The providers can be: functions, variables, literal values,
// Collections, *Collections, or Providers.
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
// with *Provider, named functions, anonymous functions, and literal values.
func Sequence(name string, providers ...any) *Collection {
	return newCollection(name, providers...)
}

var clusterID int32 = 1

// Cluster is a variation on Sequence() with the additional behavior
// that all of the providers in the in the cluster will be included
// or excluded as a group.  This doesn't apply to providers that cannot
// be included at all.  It also downgrades providers that are in the
// cluster that would normally be considered desired because they don't
// return anything and aren't wrappers: they're no longer automatically
// considered desired because doing so would imply the entire Cluster is
// is desired.
//
// A "Cluster" with only one member is not really a cluster and will
// not be treated as a cluster.
func Cluster(name string, providers ...any) *Collection {
	c := newCollection(name, providers...)
	if len(providers) > 1 {
		id := atomic.AddInt32(&clusterID, 1)
		for _, fm := range c.contents {
			fm.cluster = id
		}
	}
	return c
}

// Append adds additional providers onto an existing collection
// to create a new collection.  The additional providers may be
// value literals, functions, Providers, or *Collections.  The original
// collection is not modified.
func (c *Collection) Append(name string, funcs ...any) *Collection {
	nc := newCollection(name, funcs...)
	contents := make([]*provider, 0, len(c.contents)+len(nc.contents))
	contents = append(contents, c.contents...)
	contents = append(contents, nc.contents...)
	nc.contents = contents
	return nc
}

// Provide wraps an individual provider.  It allows the provider
// to be named.  The return value is chainable with with annotations
// like Cacheable() and Required().  It can be included in a collection.
// When providers are not named, they get their name from their position
// in their collection combined with the name of the collection they are in.
//
// When used on an existing Provider, it creates an annotated copy of that provider.
func Provide(name string, fn any) Provider {
	return newThing(fn).modify(func(fm *provider) {
		fm.origin = name
	})
}

// TODO: add ExampleMustCache

// MustCache creates an Inject item and annotates it as required to be
// in the STATIC set.  If it cannot be placed in the STATIC set
// then any collection that includes it is invalid.
func MustCache(fn any) Provider {
	return newThing(fn).modify(func(fm *provider) {
		fm.mustCache = true
		fm.cacheable = true
	})
}

// TODO: add ExampleCacheable

// Cacheable creates an inject item and annotates it as allowed to be
// in the STATIC chain.  Without this annotation, MustCache, or
// Memoize, a provider will be in the RUN chain.
//
// When used on an existing Provider, it creates an annotated copy of that provider.
func Cacheable(fn any) Provider {
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
// MustCache().
func Singleton(fn any) Provider {
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
func NotCacheable(fn any) Provider {
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
// Memoized providers will remember every combination of imputs they
// have ever seen.  This can exhaust all memory.
//
// By default, Memozied providers are Cacheable, but that doesn't force
// the provider into the STATIC set where it runs infrequently.
// Combine Memoize with MustCache to make sure that Memoize is actually
// in the STATIC set where it probably won't exhaust all memory.
//
// Use NotCacheable to exclude Memoized providers from the STATIC set.
// Remember: they'll remember every combination of inputs.
//
// When used on an existing Provider, it creates an annotated copy of that provider.
//
// As long as consistent injection chains are used Memoize + MustCache can
// guarantee singletons.
func Memoize(fn any) Provider {
	return newThing(fn).modify(func(fm *provider) {
		fm.memoize = true
		fm.cacheable = true
	})
}

// TODO: add ExampleRequired

// Required creates a new provider and annotates it as
// required: it will be included in the provider chain even
// if its outputs are not used.
//
// When used on an existing Provider, it creates an annotated copy of that provider.
func Required(fn any) Provider {
	return newThing(fn).modify(func(fm *provider) {
		fm.required = true
	})
}

// TODO: add ExampleDesired

// Desired creates a new provider and annotates it as
// desired: it will be included in the provider chain
// unless doing so creates an un-met dependency.
//
// Injectors and wrappers that have no outputs are automatically
// considered desired.
//
// When used on an existing Provider, it creates an annotated copy of that provider.
func Desired(fn any) Provider {
	return newThing(fn).modify(func(fm *provider) {
		fm.desired = true
	})
}

// TODO: add ExampleShun

// Shun creates a new provider and annotates it as not
// desired: even if it appears to be needed because another
// provider uses its output, the chain will be built without
// it if possible.
func Shun(fn any) Provider {
	return newThing(fn).modify(func(fm *provider) {
		fm.shun = true
	})
}

// TODO: add ExampleMustConsume

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
func MustConsume(fn any) Provider {
	return newThing(fn).modify(func(fm *provider) {
		fm.mustConsume = true
	})
}

// TODO: add ExampleConsumptionOptional

// ConsumptionOptional creates a new provider and annotates it as
// allowed to have some of its return values ignored.
// Without this annotation, a wrap function will not be included
// if some of its return values are not consumed.
//
// In the downward direction, optional consumption is the default.
//
// When used on an existing Provider, it creates an annotated copy of that provider.
func ConsumptionOptional(fn any) Provider {
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
func callsInner(fn any) Provider {
	return newThing(fn).modify(func(fm *provider) {
		fm.callsInner = true
	})
}

// Parallel annotates a wrap function to indicate that
// the inner function may be invoked in parallel.
//
// At the current time, support for this is very
// limited.  Returned values cannot be propagated
// across such a call and the resulting lack of
// initialization can cause a panic.
func Parallel(fn any) Provider {
	return newThing(fn).modify(func(fm *provider) {
		fm.parallel = true
	})
}

// TODO: add ExampleLoose

// Loose annotates a wrap function to indicate that when trying
// to match types against an output or return values from this
// provider, an in-exact match is acceptable.  This matters when inputs and
// returned values are specified as interfaces.  With the Loose
// annotation, an interface can be matched to the outputs and/or
// return values of this provider if the output/return value
// implements the interface.
//
// By default, an exact match of types is required for all providers.
func Loose[T any](fn any) Provider {
	return newThing(fn).modify(func(fm *provider) {
		if fm.loose == nil {
			fm.loose = make(map[typeCode]struct{})
		}
		t := reflect.TypeOf((*T)(nil)).Elem()
		fm.loose[getTypeCode(t)] = struct{}{}
	})
}

// TODO: add ExampleNonFinal

// NonFinal annotates a provider to say that it shouldn't be considered the
// final provider in a list of providers.  This is to make it possible to
// insert a provider into a list of providers late in the chain without
// actually being the final provider.  It's easy to insert a final at the
// start of the chain -- you simply list it first.  It's easy to insert a
// final provider.  Without NonFinal, it's hard or impossible to insert
// a provider very late in the chain.  If NonFinal providers are invoked,
// they will be called before the final provider.
func NonFinal(fn any) Provider {
	return newThing(fn).modify(func(fm *provider) {
		fm.nonFinal = true
	})
}

// AllowReturnShadowing annotates a provider (assumed to be a
// wrapper/middleware) to say that it is okay for the wrapper to return
// a type (usually error) that it does not in turn receive from
// its call to inner(). This is only a footgun (and error) if a downstream
// provider returns that type.
//
// A wrapper that returns a type overrides the value for that type that
// was returned from further down the chain. To preserve the value from
// down the chain, the wrapper must recevie the value in the return values
// from the inner function that it calls.
//
// For example, in the following chain, "footgun" can lose the error
// from the return to callSomething()
//
//	 err := nject.Run("bad",
//		nject.Provide("footgun", func(inner func()) error {
//			if someCondition {
//				return fmt.Errorf("some condition happened")
//			}
//			inner()
//			return nil
//		}),
//		func() error {
//			return callSomething()
//		},
//	)
//
// Starting with version 2.0, nject rejects chains with shadowing.
func AllowReturnShadowing[T any](fn any) Provider {
	return newThing(fn).modify(func(fm *provider) {
		if fm.shadowingAllowed == nil {
			fm.shadowingAllowed = make(map[typeCode]struct{})
		}
		t := reflect.TypeOf((*T)(nil)).Elem()
		fm.shadowingAllowed[getTypeCode(t)] = struct{}{}
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
// of the provider chain runs.  For example, if the static portion
// of an injection chain consists of:
//
//	func(int) string { ... }
//	func(string) int64 { ... }
//
// Then the return value from the initialization could include int,
// int64, and string but no other types.
//
// Bind pre-computes as much as possible so that the invokeFunc is fast.
//
// Each call to Bind() with unique providers may leak a small amount of memory,
// creating durable type maps and closures to handle memoization and singletons.
// Calls to the invokeFunc do not leak memory except where there are new inputs to
// providers marked Memoize().
func (c *Collection) Bind(invokeFunc any, initFunc any) error {
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

func (c *Collection) bindFast(invokeFunc any, initFunc any) error {
	invokeF := newProvider(invokeFunc, -1, c.name+" invoke func")
	var initF *provider
	if initFunc != nil {
		initF = newProvider(initFunc, -1, c.name+" initialization func")
	}

	debugLock.RLock()
	defer debugLock.RUnlock()
	return doBind(c, invokeF, initF, true)
}

// SetCallback expects to receive a function as an argument.  SetCallback() will call
// that function.  That function in turn should take one or two functions
// as arguments.  The first argument must be an invoke function (see Bind).
// The second argument (if present) must be an init function.  The invoke func
// (and the init func if present) will be created by SetCallback() and passed
// to the function SetCallback calls.
//
// If there is an init function, it must be called once before the invoke function
// is ever called.  Calling the invoke function will invoke the the sequence of
// providers.
//
// Whatever arguments the invoke and init functions take will be passed into the
// chain.  Whatever values the invoke function returns must be produced by the
// injection chain.
func (c *Collection) SetCallback(setCallbackFunc any) error {
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
				reflect.ValueOf(initPtr).Elem(),
			})
		}
	} else {
		err = c.Bind(invokePtr, nil)
		if err == nil {
			setter.Call([]reflect.Value{reflect.ValueOf(invokePtr).Elem()})
		}
	}
	return err
}

// ForEachProvider iterates over the Providers within a Collection
// invoking a function.
func (c Collection) ForEachProvider(f func(Provider)) {
	for _, fm := range c.contents {
		f(fm)
	}
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
func Run(name string, providers ...any) error {
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
func MustRun(name string, providers ...any) {
	err := Run(name, providers...)
	if err != nil {
		panic(err)
	}
}

// MustBindSimple binds a collection with an invoke function that takes no
// arguments and returns no arguments.  It panic()s if Bind() returns error.
func (c *Collection) MustBindSimple() func() {
	var invoke func()
	c.MustBind(&invoke, nil)
	return invoke
}

// MustBindSimpleError binds a collection with an invoke function that takes no
func (c *Collection) MustBindSimpleError() func() error {
	var invoke func() error
	c.MustBind(&invoke, nil)
	return invoke
}

// MustBind is a wrapper for Collection.Bind().  It panic()s if Bind() returns error.
func (c *Collection) MustBind(invokeFunc any, initFunc any) {
	err := c.Bind(invokeFunc, initFunc)
	if err != nil {
		panic(DetailedError(err))
	}
}

// MustSetCallback is a wrapper for Collection.SetCallback().  It panic()s if SetCallback() returns error.
func (c *Collection) MustSetCallback(binderFunction any) {
	err := c.SetCallback(binderFunction)
	if err != nil {
		panic(DetailedError(err))
	}
}
