# nject - dependency injection 

[![Stability: Active](https://masterminds.github.io/stability/active.svg)](https://masterminds.github.io/stability/active.html)
[![GoDoc](https://godoc.org/github.com/muir/nject?status.png)](https://pkg.go.dev/github.com/muir/nject/v2)
![unit tests](https://github.com/muir/nject/actions/workflows/go.yml/badge.svg)
[![report card](https://goreportcard.com/badge/github.com/muir/nject)](https://goreportcard.com/report/github.com/muir/nject)
[![codecov](https://codecov.io/gh/muir/nject/branch/main/graph/badge.svg)](https://codecov.io/gh/muir/nject)

Install:

	go get github.com/muir/nject/v2

---

This package provides type-safe dependency injection without requiring
users to do type assertions.

Dependencies are injected via a call chain: list functions to be called
that take and return various parameters.  The functions will be called
in order using the return values from earlier functions as parameters
for later functions.  Only the functions that are needed to be called in
order to be able to call the last function in the chain are invoked.  Then
the last function in the chain is called.

Parameters are identified by their types.  To have two different `int`
parameters, define custom types.

Type safety is checked before any functions are called.

Functions whose outputs are not used are not called.  Functions may
"wrap" the rest of the list so that they can choose to invoke the
remaining list zero or more times.

Chains may be pre-compiled into closures so that they have very little
runtime penealty.

### example

	func example() {
		// Sequences can be reused.
		providerChain := Sequence("example sequence",
			// Constants can be injected.
			"a literal string value",
			// This function will be run if something downstream needs an int
			func(s string) int {
				return len(s)
			})
		Run("example",
			providerChain,
			// The last function in the list is always run.  This one needs
			// and int and a string.  The string can come from the constant
			// and the int from the function in the provider chain.
			func(i int, s string) {
				fmt.Println(i, len(s))
			})
	}

### Main APIs

Nject provides two main APIs: Bind() and Run().

Bind() is used when performance matters: given a chain of providers,
it will write two functions: one to initialize the chain and another to
invoke it.  As much as possible, all dependency injection work is done
at the time of binding and initialization so that the invoke function
operates with very little overhead.  The chain is initialized when the
initialize function is called.  The chain is run when the invoke function
is called.  Bind() does not run the chain.

Run() is used when ad-hoc injection is desired and performance is not
critical.  Run is appropriate when starting servers and running tests.
It is not recommended for HTTP endpoint handlers.  Run exectes the
chain immediately.

### Identified by type

Rather than naming values, inputs and outputs are identified by their types.  
Since Go makes it easy to create new types, this turns out to be quite easy to use.

### Types of providers

Multiple types of providers are supported.

#### Literal values

You can provide a constant if you have one.

#### Injectors

Regular functions can provide values.  Injectors will be called at
initialization time when they're marked as cacheable or at invocation
time if they're not.

Injectors can be memoized.

Injectors can return a special error type that stops the chain.

Injectors can use data produced by earlier injectors simply by having
a function parameter that matches the type of a return value of an
earlier injector.

Injector chains can be collapsed to become a single injector -- this
allows the chain to branch out in a tree structure so that it can 
generate multiple copies of the same type.

Injectors can be automatically reordered so that outputs are generated
before they're required as inputs.

#### Wrappers (aka Middleware)

Wrappers are special functions that are responsible for invoking
the part of the injection chain that comes after themselves.  They
do this by calling an `inner()` function that the nject framework
defines for them.

Any arguments to the inner() function are injected as values available
further down the chain.  Any return values from inner() must be returned
by the final function in the chain or from another wrapper further down
the chain.

### Composition

Collections of injectors may be composed by including them in
other collections.

## Debugging injector chains

If you chain successfully binds but does not do what you expect, add
something into your chain that receives the `nject.Debugging` type:

	func(d *nject.Debugging) {
		fmt.Println("Injectors included\n", d.Included)
	}

If you chain does not bind, then `Debugging` won't help.
Injection chain errors attempt to be self-explanatory, but sometimes that's not enough.

If you're building your injection sequence dynamically, it may be useful to print
the injection chain.  It has a `String()` method.

If you think that the issue is with nject, then open an issue with the reproduce information
that is available as part of the debugging (`Debugging.Reproduce`) or available by calling
`nject.DetailedError(err)` on the error returned from `Bind()` or `Run()`.

# Uses

Nject has been successfully used for:

- Test frameworks so that test authors specify the test dependencies with the types
  injected into a function that runs a test.  Using wrappers or `t.Clanup()` in the
  injection chain allows test writers to skip all or most test cleanup.
- Endpoint handlers.  Nject makes middleware easier to write.
- Server startup.

# Related packages

The following use nject to provide nicer APIs:

- [nape](https://github.com/muir/nape): dependency injection wrappers for binding http endpoint handlers using gorilla/mux
- [nchi](https://github.com/muir/nchi): http router on top of [httprouter](https://pkg.go.dev/github.com/julienschmidt/httprouter) with a go-chi-like API
- [nfigure](https://github.com/muir/nfigure): configuration and flag processing
- [npoint](https://github.com/muir/npoint): dependency injection wrappers for binding http endpoint handlers
- [nserve](https://github.com/muir/nserve): injection chains for for starting and stopping servers
- [ntest](https://github.com/memsql/ntest): support functions and hints for using nject to write tests
- [nvalid](https://github.com/muir/nvalid): enforce that http endpoints conform to Swagger definitions
- [nvelope](https://github.com/muir/nvelope): injection chains for building endpoints

### Development status

The core of nject is stable and reliable.  New features have tests, but may not have
enough production use to be reliable.

This repo represents continued development of Blue Owl's 
[nject](https://github.com/BlueOwlOpenSource/nject) base.

Nject will follow strict semver behavior except (1) things noted as experimental;
and (2) releases that are less than 10 days old may have corrections applied that
are not strictly backwards compatible with the recent release (for example, 1.0.0
accidentally exported a type that should have remained private).

Prior to release 0.2.0, nject was bundled with other packages.  Those
other packages are now in their own repos: 
[npoint](https://github.com/muir/npoint),
[nserve](https://github.com/muir/nserve), and
[nvelope](https://github.com/muir/nvelope).  Additionally, npoint
was split apart so that the gorilla dependency is separate and is in
[nape](https://github.com/muir/nape).

Please create issues for any suggested features, improvements, or problems you run into.

