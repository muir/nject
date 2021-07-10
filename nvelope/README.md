# nvelope - http endpoint helpers in an nject world

[![GoDoc](https://godoc.org/github.com/muir/nject/nserver?status.png)](http://godoc.org/github.com/muir/nject/nvelope)

Install:

	go get github.com/muir/nject

---

This package provides helpers wrapping endpoints.  


## Typical chain

A typical endpoint wrapping chan contains some or all of the following

### Deferred Writer

Use `nvelope.InjectWriter` to create a `DeferredWriter`.  A `DeferredWriter` is a
useful enchancement to `http.ResponseWriter` that allows the output to be reset and
allows headers to be set at any time.  The cost of a `DeferredWriter` is that 
the output is buffered and copied.

### Create a logger

A logger will be used by many of the parts later in the chain.  This must be 
injected wrapped with `nject.Loose` so as to not constain the actual logger.

The logger must implement at least the following interface:

```go
type MyLogger interface{
}
```

### Grab the request body

### Marshal response

We need the request encoder this early in the framework
so that it can marshal error responses.

A JSON marshaller is provided: `nvelope.JSON`.

### Validate response

This is a user-provided optional step that can be used to double-check
that what is being sent matches the API defintion.

### Check for errors

How to handle errors is likely to be customized so the error responder
provided with `nvelope` is very simple and should probably be overridden.

Here's what's provded:

```go
var CatchErrors = nject.Provide("catch-errors", 
	func(inner func() (Any, error), w DeferredWriter) Any {
		model, err := inner()
		if err != nil {
			w.WriteHeader(500)
			return map[string]string{
				"Error": err.Error()
			}
		}
		return model
	})
```

### Validate the request

This is an optional step, provided by the user of `nvelope`, that 
should return `nject.TerminalError` if the request is not valid.  Other
validation can happen later, but this is good place to enforce API compliance.

