# nvelope - http endpoint helpers in an nject world

[![GoDoc](https://godoc.org/github.com/muir/nject/nserver?status.png)](http://godoc.org/github.com/muir/nject/nvelope)

Install:

	go get github.com/muir/nject

---

This package provides helpers for wrapping endpoints.

## Typical chain

A typical endpoint wrapping chan contains some or all of the following.

### Create a logger

This is an option step that is recommended if you're using request-specific
loggers.  The encoding provider can uses a logger that implements the 
`nvelope.BasicLogger` interface.  `nvelope.LoggerFromStd` can create
an `nvelope.BasicLogger` from the "log" logger.  `nvelope.NoLogger` provides
a `nvelope.BasicLogger` that does nothing.

### Deferred Writer

Use `nvelope.InjectWriter` to create a `*DeferredWriter`.  A `*DeferredWriter` is a
useful enchancement to `http.ResponseWriter` that allows the output to be reset and
allows headers to be set at any time.  The cost of a `*DeferredWriter` is that 
the output is buffered and copied.

### Marshal response

We need the request encoder this early in the framework
so that it can marshal error responses.

A JSON marshaller is provided: `nvelope.EncodeJSON`.  Other
response encoders can be created with `nvelope.MakeResponseEncoder`.

### Catch panics

Have the endpoint return a 500 when there is a panic.  
`nvelope.SetErrorOnPanic()` is a function that can be deferred to 
notice a panic and create a useful error.  In an injection
chain, use `nvelpe.CatchPanic`.

### Return 204 for nil responses

Use an extra injector to trigger a 204 response for nil content instead
of having the encoder handle nil specially.  `nvelope.Nil204` does this.

### Grab the request body

The request body is more convieniently handled as a []byte .  This is also
one place where API enforcement can be done.  The type `nvelope.Body` is provided by
`nvelope.ReadBody` via injection to any provider that wants it.

### Validate response

This is a user-provided optional step that can be used to double-check
that what is being sent matches the API defintion.

### Decode the request body

The request body needs to be unpacked with an unmarshaller of some kind.
`nvelope.GenerateDecoder` creates decoders that examine the injection chain
looking for models that are consumed but not provided.  If it finds any,
it examines those models for struct tags that indicate that nvelope should
create and fill the model.

If so, it generates a provider that fills the model from the request.
This includes filling fields for the main decoded request body and also
includes filling fields from URL path elements, URL query parameters, and
HTTP headers.

`nvelope.DecodeJSON` and `nvelope.DecodeXML` are pre-defined for
convience.

### Validate the request

This is an optional step, provided by the user of `nvelope`, that 
should return `nject.TerminalError` if the request is not valid.  Other
validation can happen later, but this is good place to enforce API compliance.

### Actually handle the request

At this point the request model has been decoded.  The other input parameters
(from headers, path, and query parameters) have been decoded.  The input model
may have been validated.

The response will automatically be encoded.  The endpoint handler returns the
response and and error.  If there is an error, it will trigger an appropriate
return.  Use `nvelope.ReturnCode` to set the return code if returning an error.

