// Stuff

/*

Package npoint is a general purpose lightweight non-opinionated web
server framework that provides a concise way to handle errors and inject
dependencies into http endpoint handlers.

Why

Composite endpoints: endpoints are assembled from a collection of handlers.

Before/after actions: the middleware handler type wraps the rest of the
handler chain so that it can both inject items that are used downstream
and process return values.

Dependency injection: injectors, static injectors, and fallible injectors
can all be used to provide data and code to downstream handlers.  Downstream
handlers request what they need by including appropriate types in their
argument lists.  Injectors are invoked only if their outputs are consumed.

Code juxtaposition: when using pre-registered services, endpoint binding can
be registered next to the code that implements the endpoint even if the endpoints
are implemented in multiple files and/or packages.

Delayed initialization: initializers for pre-registered services are only executed
when the service is started and bound to an http server.  This allow code to define
such endpoints to depend on resources that may not be present unless the service
is started.

Reduced refactoring cost: handlers and endpoints declare their inputs and outputs
in the argument lists and return lists.  Handlers only need to know about their own
inputs and outputs.  The endpoint framework carries the data to where it is needed.
It does so with a minimum of copies and without recursive searches (see context.Context).
Type checking is done at service start time (or endpoint binding time when binding to
services that are already running).

Lower overhead indirect passing: when using context.Context to pass values indirectly,
the Go type system cannot be used to verify types at compile time or startup time.  Endpoint
verifies types at startup time allowing code that receives indirectly-passed data simpler.
As much as possible, work is done at initialization time rather than endpoint invocation time.

Basics

To use the npoint package, create services first.  After that the
endpoints can be registered to the service and the service can be started.

A simpler way to use endpoint is to use the CreateEndpoint function.  It
converts a list of handlers into an http.HandlerFunc.  This bypasses service
creation and endpoint registration.
See https://github.com/BlueOwlOpenSource/npoint/blob/master/README.md
for an example.

Terminology

Service is a collection of endpoints that can be started together and may share
a handler collection.

Handler is a function that is used to help define an endpoint.

Handler collection is a group of handlers.

Downstream handlers are handlers that are to the right of the current handler
in the list of handlers.  They will be invoked after the current handler.

Upstream handlers are handlers that are to the left of the current handler
in the list of handlers.  They will have already been invoked by the time the
current handler is invoked.

Services

A service allows a group of related endpoints to be started together.
Each service may have a set of common handlers that are shared among
all the endpoints registered with that service.

Services come in four flavors: started or pre-registered; with Mux or
with without.

Pre-registered services are not initialized until they are Start()ed.  This
allows them to depend upon resources that may not may not be available without
causing a startup panic unless they're started without their required resources.
It also allows endpoints to be registered in init() functions next to the
definition of the endpoint.

Handlers

The handlers are defined using the nject framework:
See https://github.com/BlueOwlOpenSource/nject/blob/master/README.md

A list of handlers will be invoked from left-to-right.  The first
handler in the list is invoked first and the last one (the endpoint)
is invoked last.  The handlers do not directly call each other --
rather the framework manages the invocations.  Data provided by one
handler can be used by any handler to its right and then as the
handlers return, the data returned can be used by any handler to its
left.  The data provided and required is identified by its type.
Since Go makes it easy to make aliases of types, it is easy to make
types distinct.  When there is not an exact match of types, the framework
will use the closest (in distance) value that can convert to the
required type.

Each handler function is distinguished by its position in the
handler list and by its primary signature: its arguments
and return values.  In Go, types may be named or unnamed.  Unnamed function
types are part of primary signature.  Named function types are not part
of the primary signature.

These are the types that are recognized as valid handlers:
Static Injectors, Injectors, Endpoints, and Middleware.

Injectors are only invoked if their output is consumed or they have
no output.  Middleware handlers are (currently) always invoked.

Injectors

There are three kinds of injectors: static injectors, injectors, and
fallible injectors.

Injectors and static injectors have the following type signature:

	func(input value(s)) output values(s)

None of the input or output parameters may be un-named functions.
That describes nearly every function in Go.  Handlers that match a more
specific type signature are that type, rather than being an injector or
static injector.

Injectors whose output values are not used by a downstream handler
are dropped from the handler chain.  They are not invoked.  Injectors
that have no output values are a special case and they are always retained
in the handler chain.

Static injectors are called exactly once per endpoint.  They are called
when the endpoint is started or when the endpoint is registered -- whichever
comes last.

Values returned by static injectors will be shared by all invocations of
the endpoint.

Injectors are called once per endpoint invocation (or more if they are
downstream from a middleware handler that calls inner() more than once).

Injectors a distingued from static injectors by either their position in
the handler list or by the parameters that they take.  If they take
http.ResponseWriter or *http.Request, then they're not static.  Anything
that is downstream of a non-static injector or middleware handler is also
not static.

Fallible injectors are injectors whose first return values is of type
nject.TerminalError:

	func(input value(s)) (nject.TerminalError, output values(s))

If a non-nil value is returned as the nject.TerminalError from a fallible
injector, none of the downstream handlers will be called.  The handler
chain returns from that point with the nject.TerminalError as a return
value.  Since all return values must be consumed by a middleware handler,
fallible injectors must come downstream from a middleware handler that
takes nject.TerminalError as a returned value.  If a fallible injector returns
nil for the nject.TerminalError, the other output values are made available
for downstream handlers to consume.  The other output values are not
considered return values and are not available to be consumed by upstream
middleware handlers.

Some examples:

	func staticInjector(i int, s string) int { return i+7 }

	func injector(r *http.Request) string { return r.FormValue("x") }

	func fallibleInjector(i int) nject.TerminalError {
		if i > 10 {
			return fmt.Errorf("limit exceeded")
		}
		return nil
	}

Middleware handlers

Middleware handlers wrap the handlers downstream in a inner() function that they
may call.  The type signature of a middleware handler is a function that
receives an function as its first parameter.  That function must be of an
anonymous type:

	// middleware handler
	func(innerfunc, input value(s)) return value(s)

	// innerfunc
	func(output value(s)) returned value(s)

For example:

	func middleware(inner func(string) int, i int) int {
		j := inner(fmt.Sprintf("%d", i)
		return j * 2
	}

When this middleware function runs, it is responsible for invoking
the rest of the handler chain.  It does this by calling inner().
The parameters to inner are available as inputs to downstream
handlers.  The value(s) returned by inner come from the return
values of downstream middleware handlers and the endpoint handler.

Middleware handlers can call inner() zero or more times.

The values returned by middleware handlers must be consumed by another
upstream middlware handler.

Endpoint Handlers

Endpoint handlers are simply the last handler in the handler chain.
They look like regular Go functions.  Their input parameters come
from other handlers.  Their return values (if any) must be consumed by
an upstream middleware handler.

	func(input value(s)) return values(s)

Panics

Endpoint will panic during endpoint registration if the provided handlers
do not constitute a valid chain.  For example, if a some handler requires
a FooType but there is no upstream handler that provides a FooType then
the handler list is invalid and endpoint will panic.

Endpoint should not panic after initialization.

*/
package npoint
