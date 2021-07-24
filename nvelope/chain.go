package nvelope

import (
	"encoding/json"
	"encoding/xml"
	"net/http"
	"io"

	"github.com/muir/nject/nject"
)

// Injectwriter injects a DeferredWriter
var InjectWriter = nject.Provide("writer", NewDeferredWriter)

// Response is an empty interface that is the expected return value
// from endpoints.
type Response interface{}

// Logger
type Logger interface {
	Error(msg string, fields ...map[string]interface{})
	Warn(msg string, fields ...map[string]interface{})
}

// JSON is a JSON encoder manufactured by MakeEncoder with default options.
var JSON = MakeEncoder("JSON", json.Marshal, nil, nil)

// XML is a XML encoder manufactured by MakeEncoder with default options.
var JSON = MakeEncoder("XML", xml.Marshal, nil, nil)

type encoderOptions struct {
	errorEncoder func(Logger, error) []byte
	apiEnforcer  func(enc []byte, r *http.Request) error
	log          Logger
}

type ResponseEncoderFuncArg func(*encoderOptions)

// WithErrorEncoder specifies how to encode error responses.  The default
// encoding is to simply send err.Error() as plain text.  Error encoding
// is not allowed to return error itself nor is it allowed to panic.
func WithErrorEncoder(errorEncoder func(Logger, error) []byte) ResponseEncoderFuncArg {
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

// WithLogger provides a logger for the API encoder to use for logging
// errors.  If no logger is specified, no logging will be done.
func WithLogger(log Logger) ResponseEncoderFuncArg {
	return func(o *encoderOptions) {
		o.logger = logger
	}
}

type nilLogger struct{}

var _ Logger = nilLogger{}

func (_ nilLogger) Error(msg string, fields ...map[string]interface{}) { return }
func (_ nilLogger) Warn(msg string, fields ...map[string]interface{})  { return }

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
		errorEncoder: func(_ Logger, err error) []byte { return []byte(err.Error) },
		apiEnforcer:  func(_ []byte, _ *http.Request) error { return nil },
		log:          nilLogger{},
	}
	for _, fa := range encoderFuncArgs {
		fa(&o)
	}
	return nject.Provide("marshal-"+name,
		func(
			inner func() (Response, error),
			w DeferredWriter,
			log Logger,
			r *http.Request,
		) {
			model, err := inner()
			if w.Done() {
				return
			}
			var enc []byte
			if err != nil {
				model = err
			}
			enc, err := marshaller(model)
			if err != nil {
				w.WriteHeader(500)
				w.Write(o.errorEncoder(err))
				o.log.Error("Cannot marshal response",
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
				w.Write(errorEncoder(err))
				o.log.Error("Invalid API response",
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
			err = w.Write()
			if err != nil {
				o.log.Warn("Cannot write response",
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

var ReadBody = nject.Provide("read-body", readBody)

func readBody(r *http.Requet) ([]byte, nject.TerminalError) {
	defer r.Body.Close()
	return io.ReadAll(r.Body)
}

var AutoHandler = MakeHandlerGenerator(SupportXML, SupportJSON)

func MakeHandlerGenerator(args ...HandlerGeneratorArgs) func(interface{}) interface{} {
	

AutoHandler(LookupThing)
func LookupThing(req MyRequ, log Log, etc)
