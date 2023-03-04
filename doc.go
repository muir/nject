// Obligatory // comment

/*

Package nject is a general purpose dependency injection framework.
It provides wrapping, pruning, and indirect variable passing.  It is type safe
and using it requires no type assertions.  There are two main injection APIs:
Run and Bind.  Bind is designed to be used at program initialization and
does as much work as possible then rather than during main execution.

List of providers

The API for nject is a list of providers (injectors) that are run in order.
The final function in the list must be called.  The other functions are called
if their value is consumed by a later function that must be called.  Here
is a simple example:

	func main() {
		nject.Run("example",
			context.Background, // provides context.Context
			log.Default,        // provides *log.Logger
			":80",              // a constant string
			http.NewServeMux,   // provides *http.ServeMux
			func(mux *http.ServeMux) http.Handler {
				mux.HandleFunc("/missing", http.NotFound)
				return mux
			},
			http.ListenAndServe, // uses a string and http.Handler
		)
	}

In this example, context.Background and log.Default are not invoked because
their outputs are not used by the final function (http.ListenAndServe).

How to use

The basic idea of nject is to assemble a Collection of providers and then use
that collection to supply inputs for functions that may use some or all of
the provided types.

One big win from dependency injection with nject is the ability to
reshape various different functions into a single signature.  For example,
having a bunch of functions with different APIs all bound as http.HandlerFunc
is easy.

Providers produce or consume data. The data is distinguished by its
type.  If you want to three different strings, then define three different
types:

	type myFirst string
	type mySecond string
	type myThird string

Then you can have a function that does things with the three types:

	func myStringFunc(first myFirst, second mySecond) myThird {
		return myThird(string(first) + string(second))
	}

The above function would be a valid injector or final function in a
provider Collection.  For example:

	var result string
	Sequence("example sequence",
		func() mySecond {
			return "2nd"
		}
		myStringFunc,
	).Run("example run",
		func(s myThird) {
			result = string(s)
		},
		myFirst("1st"))
	fmt.Println(result)

This creates a sequence and executes it.  Run injects a myFirst value and
the sequence of providers runs: genSecond() injects a mySecond and
myStringFunc() combines the myFirst and mySecond to create a myThird.
Then the function given in run saves that final value.  The expected output
is

	1st2nd

Collections

Providers are grouped as into linear sequences.  When building an injection chain,
the providers are grouped into several sets: LITERAL, STATIC, RUN.  The LITERAL
and STATIC sets run once per initialization.  The RUN set runs once per invocation.  Providers
within a set are executed in the order that they were originally specified.
Providers whose outputs are not consumed are omitted unless they are marked Required().

Collections are bound with Bind(&invocationFunction, &initializationFunction).  The
invocationFunction is expected to be used over and over, but the initializationFunction
is expected to be used less frequently.  The STATIC set is re-invoked each time the
initialization function is run.

The LITERAL set is just the literal values in the collection.

The STATIC set is composed of the cacheable injectors.

The RUN set if everything else.

Injectors

All injectors have the following type signature:

	func(input value(s)) output values(s)

None of the input or output parameters may be anonymously-typed functions.
An anoymously-typed function is a function without a named type.

Injectors whose output values are not used by a downstream handler
are dropped from the handler chain.  They are not invoked.  Injectors
that have no output values are a special case and they are always retained
in the handler chain.

Cached injectors

In injector that is annotated as Cacheable() may promoted to the STATIC set.
An injector that is annotated as MustCache() must be promoted to
the STATIC set: if it cannot be promoted then the collection is deemed invalid.

An injector may not be promoted to the STATIC set if it takes as
input data that comes from a provider that is not in the STATIC or
LITERAL sets.  For example, arguments to the invocation function,
if the invoke function takes an int as one of its inputs, then no
injector that takes an int as an argument may be promoted to the
STATIC set.

Injectors in the STATIC set will be run exactly once per set of input values.
If the inputs are consistent, then the output will be a singleton.  This is
true across injection chains.

If the following provider is used in multiple chains, as long as the same integer
is injected, all chains will share the same pointer.

	Provide("square", MustCache(func(int i) *int {
		j := i*i
		return &j
	}))

Memoized injectors

Injectors in the STATIC set are only run for initialization.  For some things,
like opening a database, that may still be too often.  Injectors that are marked
Memoized must be promoted to the static set.

Memoized injectors are only run once per combination of inputs.   Their outputs
are remembered.  If called enough times with different arguments, memory will
be exhausted.

Memoized injectors may not have more than 90 inputs.

Memoized injectors may not have any inputs that are go maps, slices, or functions.
Arrays, structs, and interfaces are okay.  This requirement is recursive so a struct that
that has a slice in it is not okay.

Fallible injectors

Fallible injectors are special injectors that change the behavior of the injection
chain if they return error.  Fallible injectors in the RUN set, that return error
will terminate execution of the injection chain.

A non-wrapper function that returns nject.TerminalError is a fallible injector.

	func(input value(s)) (output values(s), TerminalError)

The TerminalError does not have to be the last return value.  The nject
package converts TerminalError objects into error objects so only the
fallible injector should use TerminalError.  Anything that consumes the
TerminalError should do so by consuming error instead.

Fallible injectors can be in both the STATIC set and the RUN set.  Their
behavior is a bit different.

If a non-nil value is returned as the TerminalError from a fallible
injector in the RUN set, none of the downstream providers will be called.  The
provider chain returns from that point with the TerminalError as a return
value.  Since all return values must be consumed by a middleware provider or
the bound invoke function,
fallible injectors must come downstream from a middleware handler that
takes error as a returned value if the invoke function (function that runs
a bound injection chain) does not return error.  If a fallible injector returns
nil for the TerminalError, the other output values are made available
for downstream handlers to consume.  The other output values are not
considered return values and are not available to be consumed by upstream
middleware handlers.  The error returned by a fallible injector is not available
downstream.

If a non-nil value is returned as the TerminalError from a fallible
injector in the STATIC set, the rest of the STATIC set will be skipped.
If there is an init function and it returns error, then the value returned
by the fallible injector will be returned via init function.  Unlike
fallible injectors in the RUN set, the error output by a fallible injector
in the STATIC set is available downstream (but only in the RUN set -- nothing
else in the STATIC set will execute).

Some examples:

	func staticInjector(i int, s string) int { return i+7 }

	func injector(r *http.Request) string { return r.FormValue("x") }

	func fallibleInjector(i int) nject.TerminalError {
		if i > 10 {
			return fmt.Errorf("limit exceeded")
		}
		return nil
	}

Wrap functions and middleware

A wrap function interrupts the linear sequence of providers.  It may or may
invoke the remainder of the sequence that comes after it.  The remainder of
the sequence is provided to the wrap function as a function that it may call.
The type signature of a wrap function is a function that
receives an function as its first parameter.  That function must be of an
anonymous type:

	// wrapFunction
	func(innerFunc, input value(s)) return value(s)

	// innerFunc
	func(output value(s)) returned value(s)

For example:

	func wrapper(inner func(string) int, i int) int {
		j := inner(fmt.Sprintf("%d", i)
		return j * 2
	}

When this wrappper function runs, it is responsible for invoking
the rest of the provider chain.  It does this by calling inner().
The parameters to inner are available as inputs to downstream
providers.  The value(s) returned by inner come from the return
values of other wrapper functions and from the return value(s) of
the final function.

Wrap functions can call inner() zero or more times.

The values returned by wrap functions must be consumed by another
upstream wrap function or by the init function (if using Bind()).

Wrap functions have a small amount of runtime overhead compared to
other kinds of functions: one call to reflect.MakeFunc().

Wrap functions serve the same role as middleware, but are usually
easier to write.

Final functions

Final functions are simply the last provider in the chain.
They look like regular Go functions.  Their input parameters come
from other providers.  Their return values (if any) must be consumed by
an upstream wrapper function or by the init function (if using Bind()).

	func(input value(s)) return values(s)

Wrap functions that return error should take error as a returned value so that
they do not mask a downstream error.  Wrap functions should not return TerminalError
because they internally control if the downstream chain is called.

	func GoodExample(inner func() error) error {
		if err := DoSomething(); err != nil {
			// skip remainder of injection chain
			return err
		}
		err := inner()
		return err
	}

	func BadExampleMasksDownstreamError(inner func()) error {
		if err := DoSomething(); err != nil {
			// skip remainder of injection chain
			return err
		}
		inner()
		// nil is returned even if a downsteam injector returns error
		return nil
	}

Literal values

Literal values are values in the provider chain that are not functions.

Invalid provider chains

Provider chains can be invalid for many reasons: inputs of a type not
provided earlier in the chain; annotations that cannot be honored
(eg. MustCache & Memoize); return values that are not consumed;
functions that take or return functions with an anymous type other than
wrapper functions; A chain that does not terminate with a function; etc.
Bind() and Run() will return error when presented with an invalid provider chain.

Panics

Bind() and Run() will return error rather than panic.  After Bind()ing
an init and invoke function, calling them will not panic unless a provider
panic()s

A wrapper function can be used to catch panics and turn them into errors.
When doing that, it is important to propagate any errors that are coming up
the chain.  If there is no guaranteed function that will return error, one
can be added with Shun().

	func CatchPanic(inner func() error) (err error) {
		defer func() {
			if r := recover(); r != nil {
				if e, ok := r.(error); ok {
					err = errors.Wrapf(e, "panic error from %s",
						string(debug.Stack()))
				} else {
					err = errors.Errorf("panic caught!\n%s\n%s",
						fmt.Sprint(r),
						string(debug.Stack()))
				}
			}
		}()
		return inner()
	}

	var ErrorOfLastResort = nject.Shun(func() error { return nil })

Chain evaluation

Bind() uses a complex and somewhat expensive O(n^2) set of rules to evaluate
which providers should be included in a chain and which can be dropped.  The goal
is to keep the ones you want and remove the ones you don't want.  Bind() tries
to figure this out based on the dependencies and the annotations.

MustConsume, not Desired:
Only include if at least one output is transitively consumed by a
Required or Desired chain element and all outputs are consumed by
some other provider.

Not MustConsume, not Desired: only include if at least one output
is transitively consumed by a Required or Desired provider.

Not MustConsume, Desired:
Include if all inputs are available.

MustConsume, Desired:
Only include if all outputs are transitively consumed by a required
or Desired chain element.

When there are multiple providers of a type, Bind() tries to get it
from the closest provider.

Providers that have unmet dependencies will be eliminated from the chain
unless they're Required.

Best practices

The remainder of this document consists of suggestions for how to use nject.

Contributions to this section would be welcome.  Also links to blogs or other
discussions of using nject in practice.

For tests

The best practice for using nject inside a large project is to have a few
common chains that everyone imports.

Most of the time, these common chains will be early in the sequence of
providers.  Customization of the import chains happens in many places.

This is true for services, libraries, and tests.

For tests, a wrapper that includes the standard chain makes it easier
to write tests.

	var CommonChain = nject.Sequence("common",
		context.Background,
		log.Default,
		things,
		used,
		in,
		this,
		project,
	)

	func RunTest(t *testing.T, testInjectors ...any) {
		err := nject.Run("RunTest",
			t,
			CommonChain,
			nject.Sequence(t.Name(), testInjectors...))
		assert.NoError(t, err, nject.DetailedError(err))
	}

	func TestSomething(t *testing.T) {
		t.RunTest(t, Extra, Things, func(
			ctx context.Context,
			log *log.Logger,
			etc Etcetera,
		) {
			assert.NotNil(t, ctx)
		})
	}

Displaying errors

If nject cannot bind or run a chain, it will return error.  The returned
error is generally very good, but it does not contain the full debugging
output.

The full debugging output can be obtained with the DetailedError function.
If the detailed error shows that nject has a bug, note that part of the debug
output includes a regression test that can be turned into an nject issue.
Remove the comments to hide the original type names.

	err := nject.Run("some chain", some, injectors)
	if err != nil {
		if details := nject.DetailedError(err); details != err.Error() {
			log.Println("Detailed error", details)
		}
		log.Fatal(err)
	}

Reorder

The Reorder() decorator allows injection chains to be fully or partially reordered.
Reorder is currently limited to a single pass and does not know which injectors are
ultimately going to be included in the final chain. It is likely that if you mark
your entire chain with Reorder, you'll have unexpected results.  On the other hand,
Reorder provides safe and easy way to solve some common problems.

For example: providing optional options to an injected dependency.

	var ThingChain = nject.Sequence("thingChain",
		nject.Shun(DefaultThingOptions),
		ThingProvider,
	}

	func DefaultThingOptions() []ThingOption {
		return []ThingOption{
			StanardThingOption,
		}
	}

	func ThingProvider(options []ThingOption) *Thing {
		return thing.Make(options...)
	}

Because the default options are marked as Shun, they'll only be included
if they have to be included.  If a user of thingChain wants to override
the options, they simply need to mark their override as Reorder.  To make
this extra friendly, a helper function to do the override can be provided
and used.

	func OverrideThingOptions(options ...ThingOption) nject.Provider {
		return nject.Reorder(func() []ThingOption) {
			return options
		}
	}

	nject.Run("run",
		ThingChain,
		OverrideThingOptions(thing.Option1, thing.Option2),
	)

Self-cleaning

Recommended best practice is to have injectors shutdown the things they themselves start. They
should do their own cleanup.

Inside tests, an injector can use t.Cleanup() for this.

For services, something like t.Cleanup can easily be built:

	type CleanupList struct {
		list *[]func() error

	func (l CleanupList) Cleanup(f func() error) {
		*l.list = append(*l.list, f)
	}

	func CleaningService(inner func(CleanupList) error) (finalErr error) {
		list := make([]func() error, 0, 64)
		defer func() {
			for i := len(list); i >= 0; i-- {
				err := list[i]()
				if err != nil && finalErr == nil {
					finalErr = err
				}
			}
		}()
		return inner(CleanupList{list: &list})
	}

	func ThingProvider(cleaningService CleanupList) *Thing {
		thing := things.New()
		thing.Start()
		cleaningService.Cleanup(thing.Stop)
		return thing
	}

Alternatively, any wrapper function can do it's own cleanup in a defer that it
defines.  Wrapper functions have a small runtime performance penalty, so if you
have more than a couple of providers that need cleanup, it makes sense to include
something like CleaningService.

Forcing inclusion

The normal direction of forced inclusion is that an upstream provider is required
because a downstream provider uses a type produced by the upstream provider.

There are times when the relationship needs to be reversed.  For example, a type
gets modified by a downstream injector.  The simplest option is to combine the providers
into one function.

Another possibility is to mark the upstream provider with MustConsume and have it
produce a type that is only consumed by the downstream provider.

Lastly, the providers can be grouped with Cluster so that they'll be included or
excluded as a group.

*/
package nject
