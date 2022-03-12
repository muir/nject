# Some notes on how nject is put together

## api.go

Defines most of the public APIs.  Functions transform raw providers into
annotated provider objects.

## nject.go

Definition of `provider` type.  Helper functions for manipulating
providers and collections of providers

The `provider` type is the internal structure that tracks everything to
do with an individual provider (function, constant, wrapper, etc) in
the chain.  Different fields are filled in by different parts of the
code: 

1. api.go records user annotations
2. characterize.go adds attributes related to how the provider is used
   including the input/output flows
3. bind.go adds some additional notes used for creating the closure
4. include.go keeps notes (like `include`) and generates the up/down and
   bypass maps.
5. generate.go adds per-provider closures (wrappers)

## characterize.go

Classifies providers.  This is done with a collection of predicates
and mapping from predicate sets to provider classifications.

## bind.go

Orchestrates creating the closures of `Bind()`.  `Run()` is implemnted
as a throw-away `Bind()`.

## include.go

Evaluate exactly which providers to include in the chain being created.

## generate.go

Generate a closure that evaluates the entire chain.  This includes all
of the wrapping required for the different kinds of providers.

## type_codes.go

Currently Reflect.Type is mapped to integers and the integers are used
in place of Reflect.Type everywhere.  Is this a good idea?  Maybe, maybe
not.  Reglardless, type_codes is where it happens.

## types.go

Constants for the different kinds of types and type classes.  Definiton of
the `Debugging` type.

## match.go

When types don't perfectly match, find the type that is closest.  This is needed
when a concrete type needs to used to fill a request for an interface.

## cache.go

Lookup values for Memoize() and  Singleton() providers.

## filler.go

Handles struct filling and using Reflective types.

## debug.go

Debugging can be turned on/off at runtime with thread safety.

## error.go

Custom error wrapper.
