# nject & npoint - dependency injection 

[![GoDoc](https://godoc.org/github.com/BlueOwlOpenSource/nject?status.png)](https://pkg.go.dev/github.com/BlueOwlOpenSource/nject)
[![Build Status](https://travis-ci.org/BlueOwlOpenSource/nject.svg)](https://travis-ci.org/BlueOwlOpenSource/nject)
[![report card](https://goreportcard.com/badge/github.com/BlueOwlOpenSource/nject)](https://goreportcard.com/report/github.com/BlueOwlOpenSource/nject)


Install:

	go get github.com/BlueOwlOpenSource/nject

---

This is a pair of packages:

nject: type safe dependency injection w/o requiring type assertions.

npoint: dependency injection for http endpoint handlers

### Basic idea

Dependencies are injected via a call chain: list functions to be called
that take and return various parameters.  The functions will be called
in order using the return values from earlier functions as parameters
for later functions.

Parameters are identified by their types.  To have two different int
parameters, define custom types.

Type safety is checked before any functions are called.

Functions whose outputs are not used are not called.  Functions may be
"wrap" the rest of the list so that they can choose to invoke the
remaing list zero or more times.

### nject example

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

### npoint example

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


### Minimum version

Due to the use of the "context" package, the mimimum supported Go version is 1.8.
Support for earlier versions would be easy to add if anyone cares.
