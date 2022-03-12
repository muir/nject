// Obligatory // comment

/*

Package nject is a general purpose lightweight dependency injection framework.
It provides wrapping, pruning, and indirect variable passing.  It is type safe
and using it requires no type assertions.  There are two main injection APIs:
Run and Bind.  Bind is designed to be used at program initialization and
does as much work as possible then rather than during main execution.

The basic idea is to assemble a Collection of providers and then use
that collection to supply inputs for functions that may use some or all of
the provided types.

The biggest win from dependency injection with nject is the ability to
reshape various different functions into a single signature.  For example,
having a bunch of functions with different APIs all bound as http.HandlerFunc
is easy.

Every provider produces or consumes data.  The data is distinguished by its
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
and STATIC sets run once per binding.  The RUN set runs once per invoke.  Providers
within a set are executed in the order that they were originally specified.
Providers whose outputs are not consumed are omitted unless they are marked Required().

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
An injector that is annotated as MustCache() or Memoize() must be promoted to
the STATIC set: if it cannot be promoted then the colection is deemed invalid.

An injector may not be promoted to the STATIC set if it takes as input data
that comes from a provider that is not in the STATIC or LITERAL sets.  For
example, when using Bind(), if the invoke function takes an int as one of its
inputs, then no injector that takes an int as an argument may be promoted to
the STATIC set.

Injectors in the STATIC set will be run exactly once per set of input values.
If the inputs are consistent, then the output will be a singleton.  This is
true across injection chains.

If the following provider is used in multiple chains, as long as the same integer
is injected, all chains will share the same pointer.

```go
Provide("square", MustCache(func(int i) *int {
	j := i*i
	return &j
}))
```

Memoized injectors

Injectors in the STATIC set are only run for initialization.  For some things,
like opening a database, that may still be too often.  Injectors that are marked
Memoized must be promoted to the static set.

Memoized injectors are only run once per combination of inputs.   Their outputs
are remembered.  If called enough times with different arguments, memory will
be exhausted.

Memoized injectors may not have more than 30 inputs.

Memoized injectors may not have any inputs that are go maps, slices, or functions.
Arrays, structs, and interfaces are okay.  This requirement is recursive so a struct that
that has a slice in it is not okay.

Fallible injectors

Fallible injectors are injectors that return a value of type TerminalError.

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
takes TerminalError as a returned value if the invoke function does not
return error.  If a fallible injector returns
nil for the TerminalError, the other output values are made available
for downstream handlers to consume.  The other output values are not
considered return values and are not available to be consumed by upstream
middleware handlers.  The error returned by a fallible injector is not available
downstream.

If a non-nil value is returned as the TerminalError from a fallible
injector in the STATIC set, the rest of the STATIC set will be skipped.
If there is an init function and it returns error, then the value returned
by the fallible injector will be returned via init fuction.  Unlike
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

Wrap functions

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

Final functions

Final functions are simply the last provider in the chain.
They look like regular Go functions.  Their input parameters come
from other providers.  Their return values (if any) must be consumed by
an upstream wrapper function or by the init function (if using Bind()).

	func(input value(s)) return values(s)

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

*/
package nject
