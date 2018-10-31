# npoint - dependency injection and wrapping for http handlers

[![GoDoc](https://godoc.org/github.com/BlueOwlOpenSource/npoint?status.png)](http://godoc.org/github.com/BlueOwlOpenSource/npoint)

Install:

	go get github.com/BlueOwlOpenSource/npoint

---

This package attempts to solve several issues with http endpoint handlers:

 * Chaining actions to create composite handlers 
 * Wrapping endpoint handlers with before/after actions
 * Dependency injection for endpoint handlers
 * Binding handlers to their endpoints next to the code that defines the endpoints
 * Delaying initialization code execution until services are started allowing services that are not used to remain uninitialized
 * Reducing the cost of refactoring endpoint handlers passing data directly from producers to consumers without needing intermediaries to care about what is being passed
 * Avoid the overhead of verifying that requested items are present when passed indirectly (a common problem when using context.Context to pass data indirectly)

It does this by defining endpoints as a sequence of handler functions.  Handler functions
come in different flavors.  Data is passed from one handler to the next using reflection to 
match the output types with input types requested later in the handler chain.  To the extent
possible, the handling is precomputed so there is very little reflection happening when the
endpoint is actually invoked.

Endpoints are registered to services before or after the service is started.

When services are pre-registered and started later, it is possible to bind endpoints
to them in init() functions and thus colocate the endpoint binding with the endpoint
definition and spread the endpoint definition across multiple files and/or packages.

Services come in two flavors: one flavor binds endpoints with http.HandlerFunc and the
other binds using mux.Router.  

Example uses include:

 * Turning output structs into json
 * Common error handling
 * Common logging
 * Common initialization
 * Injection of resources and dependencies

## Small Example

CreateEndpoint is the simplest way to start using the npoint framework.  It
generates an http.HandlerFunc from a list of handlers.  The handlers will be called
in order.   In the example below, first WriteErrorResponse() will be called.  It
has an inner() func that it uses to invoke the rest of the chain.  When 
WriteErrorResponse() calls its inner() function, the db injector returned by
InjectDB is called.  If that does not return error, then the inline function below
to handle the endpint is called.  

	mux := http.NewServeMux()
	mux.HandleFunc("/my/endpoint", npoint.CreateEndpoint(
		WriteErrorResponse,
		InjectDB("postgres", "postgres://..."),
		func(r *http.Request, db *sql.DB, w http.ResponseWriter) error {
			// Write response to w or return error...
			return nil
		}))

WriteErrorResponse invokes the remainder of the handler chain by calling inner().

	func WriteErrorResponse(inner func() nject.TerminalError, w http.ResponseWriter) {
		err := inner()
		if err != nil {
			w.Write([]byte(err.Error()))
			w.WriteHeader(500)
		}
	}

InjectDB returns a handler function that opens a database connection.   If the open
fails, executation of the handler chain is terminated.

	func InjectDB(driver, uri string) func() (nject.TerminalError, *sql.DB) {
		return func() (nject.TerminalError, *sql.DB) {
			db, err := sql.Open(driver, uri)
			if err != nil {
				return err, nil
			}
			return nil, db
		}
	}

