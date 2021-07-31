# nject - dependency injection 

[![GoDoc](https://godoc.org/github.com/muir/nject/nject?status.png)](http://godoc.org/github.com/muir/nject/nject)

Install:

	go get github.com/muir/nject

---

This package provides type-safe dependency injection without requiring
users to do type assertions.

### Main APIs

It provides two main APIs: Bind() and Run().

Bind() is used when performance matters: given a chain of providers,
it will write two functions: one to initialize the chain and another to
invoke it.  As much as possible, all dependency injection work is done
at the time of binding and initialization so that the invoke function
operates with very little overhead.  The chain is initialized when the
initialize function is called.  The chain is run when the invoke function
is called.  Bind() does not run the chain.

Run() is used when ad-hoc injection is desired and performance is not
critical.  Run is appropriate when starting servers and running tests.
It is not reccomended for http endpoint handlers.  Run exectes the
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

#### Wrappers

Wrappers are special functions that are responsible for invoking
the part of the injection chain that comes after themselves.  They
do this by calling an `inner()` function that the nject framework
defines for them.

Any arguments to the inner() function are injected as values available
further down the chain.  Any return values from inner() must be returned
by the final function in the chain or from another wrapper futher down
the chain.

### Composition

Collections of injectors may be composed by including them in
other collections.

