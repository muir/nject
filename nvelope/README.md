# nvelope - http endpoint helpers in an nject world

[![GoDoc](https://godoc.org/github.com/muir/nject/nserver?status.png)](http://godoc.org/github.com/muir/nject/nvelope)

Install:

	go get github.com/muir/nject

---

This package provides helpers for wrapping endpoints.  

## Typical chain

A typical endpoint wrapping chan contains some or all of the following.

### Deferred Writer

Use `nvelope.InjectWriter` to create a `DeferredWriter`.  A `DeferredWriter` is a
useful enchancement to `http.ResponseWriter` that allows the output to be reset and
allows headers to be set at any time.  The cost of a `DeferredWriter` is that 
the output is buffered and copied.

### Create a logger

This is an option step that is recommended if you're using request-specific
loggers.  The encoding provider can use a logger that implements the 
nvelope.Logger interface.

### Grab the request body

The request body is more convieniently handled as a []byte .  This is also
where API enforcement can be done.  The type nvelope.Body is provided by
nvelope.ReadBody via injection to any provider that wants it.

### Marshal response

We need the request encoder this early in the framework
so that it can marshal error responses.

A JSON marshaller is provided: `nvelope.JSON`.

### Validate response

This is a user-provided optional step that can be used to double-check
that what is being sent matches the API defintion.

### Decode the request body

The request body needs to be unpacked with an unmarshaller of some kind.
nvelope.GenerateDecoder creates decoders that examine the injection chain
looking for models that are consumed but not provided.  If it finds any,
it examines those models for struct tags that indicate that nvelope should
create and fill the model.

If so, it generates a provider that fills the model from the request.
This includes filling fields for the main decoded request body and also
includes filling fields from URL path elements, URL query parameters, and
HTTP headers.

### Check for errors

How to handle errors is likely to be customized so the error responder
provided with `nvelope` is very simple and should probably be overridden.

Here's what's provded:

XXX
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

