// Stuff

/*

Package nvelope provides injection handlers that make building
HTTP endpoints simple.  In combination with npoint and nject it
provides a API endpoint framework.

The main things it provides are a request decoder and a response
encoder.

The request decoder will fill in a struct to capture all the
parts of the request: path parameters, query parameters, headers,
and the body.  The decoding is driven by struct tags that are
interpreted at program startup.

The response encoder is comparatively simpler: given a model and an
error, it encodes the error or the model appropriately.

Deferred writer allows output to be buffered and then abandoned.

NotFound, Forbidden, and BadRequest provide easy ways to annotate
an error return to cause a specific HTTP error code to be sent.

CatchPanic makes it easy to turn panics into error returns.

The provided example puts it all together.

*/
package nvelope
