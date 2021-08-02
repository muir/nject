package nvelope

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"

	"github.com/muir/nject/nject"
)

// Injectwriter injects a DeferredWriter
var InjectWriter = nject.Provide("writer", NewDeferredWriter)

// Response is an empty interface that is the expected return value
// from endpoints.
type Response interface{}

// EncodeJSON is a JSON encoder manufactured by MakeEncoder with default options.
var EncodeJSON = MakeResponseEncoder("JSON", json.Marshal)

// EncodeXML is a XML encoder manufactured by MakeEncoder with default options.
var EncodeXML = MakeResponseEncoder("XML", xml.Marshal)

type encoderOptions struct {
	errorEncoder func(BasicLogger, error) []byte
	apiEnforcer  func(enc []byte, r *http.Request) error
}

type ResponseEncoderFuncArg func(*encoderOptions)

// WithErrorEncoder specifies how to encode error responses.  The default
// encoding is to simply send err.Error() as plain text.  Error encoding
// is not allowed to return error itself nor is it allowed to panic.
func WithErrorEncoder(errorEncoder func(BasicLogger, error) []byte) ResponseEncoderFuncArg {
	return func(o *encoderOptions) {
		o.errorEncoder = errorEncoder
	}
}

// WithAPIEnforcer specifies
// a function that can check if the encoded API response is valid
// for the endpoint that is generating the response.  This is where
// swagger enforcement could be added.  The default is not not verify
// API conformance.
func WithAPIEnforcer(apiEnforcer func(enc []byte, r *http.Request) error) ResponseEncoderFuncArg {
	return func(o *encoderOptions) {
		o.apiEnforcer = apiEnforcer
	}
}

// MakeEncoder generates an nject Provider to encode API responses.
//
// The generated provider is a wrapper that invokes the rest of the
// handler injection chain and expect to receive as return values
// an Response and and error.  If the error is not nil, then the response
// becomes the error.
func MakeResponseEncoder(
	name string,
	marshaller func(interface{}) ([]byte, error),
	encoderFuncArgs ...ResponseEncoderFuncArg,
) nject.Provider {
	o := encoderOptions{
		errorEncoder: func(_ BasicLogger, err error) []byte { return []byte(err.Error()) },
		apiEnforcer:  func(_ []byte, _ *http.Request) error { return nil },
	}
	for _, fa := range encoderFuncArgs {
		fa(&o)
	}
	return nject.Provide("marshal-"+name,
		func(
			inner func() (Response, error),
			w *DeferredWriter,
			log BasicLogger,
			r *http.Request,
		) {
			model, err := inner()
			fmt.Println("XXX ENCODE model", model)
			fmt.Println("XXX ENCODE err", err)
			if w.Done() {
				return
			}
			if err != nil {
				model = err
			}
			enc, err := marshaller(model)
			if err != nil {
				w.WriteHeader(500)
				w.Write(o.errorEncoder(log, err))
				log.Error("Cannot marshal response",
					map[string]interface{}{
						"error":  err.Error(),
						"method": r.Method,
						"uri":    r.URL.String(),
					})
				return
			}
			err = o.apiEnforcer(enc, r)
			if err != nil {
				w.WriteHeader(500)
				w.Write(o.errorEncoder(log, err))
				log.Error("Invalid API response",
					map[string]interface{}{
						"error":  err.Error(),
						"method": r.Method,
						"uri":    r.URL.String(),
					})
				return
			}
			if e, ok := model.(error); ok {
				w.WriteHeader(GetReturnCode(e))
			}
			w.Write(enc)
			err = w.Flush()
			if err != nil {
				log.Warn("Cannot write response",
					map[string]interface{}{
						"error":  err.Error(),
						"method": r.Method,
						"uri":    r.URL.String(),
					})
			}
		})
}

// Nil204 is a wrapper that causes looks for return values of Response and error
// and if both are nil, writes a 204 header and no data.  It is mean to be used
// downstream from a response encocder.
var Nil204 = nject.Provide("nil-204", nil204)

func nil204(inner func() (Response, error), w DeferredWriter) {
	model, err := inner()
	if w.Done() {
		return
	}
	if err == nil && model == nil {
		w.WriteHeader(204)
		w.Flush()
	}
}
