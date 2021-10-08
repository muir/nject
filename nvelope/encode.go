package nvelope

import (
	"encoding/json"
	"encoding/xml"
	"net/http"

	"github.com/golang/gddo/httputil"
	"github.com/muir/nject/nject"
	"github.com/pkg/errors"
)

// InjectWriter injects a DeferredWriter
var InjectWriter = nject.Provide("writer", NewDeferredWriter)

// AutoFlushWriter calls Flush on the deferred writer if it hasn't
// already been done
var AutoFlushWriter = nject.Provide("autoflush-writer", func(inner func(), w *DeferredWriter) {
	inner()
	if !w.Done() {
		_ = w.Flush()
	}
})

// Response is an empty interface that is the expected return value
// from endpoints.
type Response interface{}

// EncodeJSON is a JSON encoder manufactured by MakeResponseEncoder with default options.
var EncodeJSON = MakeResponseEncoder("JSON",
	WithEncoder("application/json", json.Marshal,
		WithEncoderErrorTransform(func(err error) (interface{}, bool) {
			var jm json.Marshaler
			if errors.As(err, &jm) {
				return jm, true
			}
			return nil, false
		}),
	))

// EncodeXML is a XML encoder manufactured by MakeResponseEncoder with default options.
var EncodeXML = MakeResponseEncoder("XML",
	WithEncoder("application/xml", xml.Marshal,
		WithEncoderErrorTransform(func(err error) (interface{}, bool) {
			var me xml.Marshaler
			if errors.As(err, &me) {
				return me, true
			}
			return nil, false
		}),
	))

type encoderOptions struct {
	encoders         map[string]specificEncoder
	contentOffers    []string
	defaultEncoder   string
	errorTransformer ErrorTranformer
}

type specificEncoder struct {
	apiEnforcer      func(httpCode int, enc []byte, header http.Header, r *http.Request) error
	errorTransformer ErrorTranformer
	encode           func(interface{}) ([]byte, error)
}

// ResponseEncoderFuncArg is a function argument for MakeResponseEncoder
type ResponseEncoderFuncArg func(*encoderOptions)

// EncoderSpecificFuncArg is a functional arguemnt for WithEncoder
type EncoderSpecificFuncArg func(*specificEncoder)

// ErrorTranformer transforms an error into a model that can be logged.
type ErrorTranformer func(error) (replacementModel interface{}, useReplacement bool)

// WithEncoder adds an model encoder to what MakeResponseEncoder will support.
// The first encoder added becomes the default encoder that is used if there
// is no match between the client's Accept header and the encoders that
// MakeResponseEncoder knows about.
func WithEncoder(contentType string, encode func(interface{}) ([]byte, error), encoderOpts ...EncoderSpecificFuncArg) ResponseEncoderFuncArg {
	return func(o *encoderOptions) {
		if o.defaultEncoder == "" {
			o.defaultEncoder = contentType
		}
		se := specificEncoder{
			encode:      encode,
			apiEnforcer: func(_ int, _ []byte, _ http.Header, _ *http.Request) error { return nil },
		}
		for _, eo := range encoderOpts {
			eo(&se)
		}
		if _, ok := o.encoders[contentType]; !ok {
			o.contentOffers = append(o.contentOffers, contentType)
		}
		o.encoders[contentType] = se
	}
}

// WithErrorModel provides a function to transform errors before
// encoding them using the normal encoder.  The return values are the model
// to use instead of the error and a boolean to indicate that the replacement
// should be used.  If the boolean is false, then a plain text error
// message will be generated using err.Error().
func WithErrorModel(errorTransformer ErrorTranformer) ResponseEncoderFuncArg {
	return func(o *encoderOptions) {
		o.errorTransformer = errorTransformer
	}
}

// WithEncoderErrorTransform provides an encoder-specific function to
// transform errors before
// encoding them using the normal encoder.  The return values are the model
// to use instead of the error and a boolean to indicate that the replacement
// should be used.  If the boolean is false, then a plain text error
// message will be generated using err.Error().
func WithEncoderErrorTransform(errorTransformer ErrorTranformer) EncoderSpecificFuncArg {
	return func(o *specificEncoder) {
		o.errorTransformer = errorTransformer
	}
}

type APIEnforcerFunc func(httpCode int, enc []byte, header http.Header, r *http.Request) error

// WithAPIEnforcer specifies
// a function that can check if the encoded API response is valid
// for the endpoint that is generating the response.  This is where
// swagger enforcement could be added.  The default is not not verify
// API conformance.
func WithAPIEnforcer(apiEnforcer APIEnforcerFunc) EncoderSpecificFuncArg {
	return func(o *specificEncoder) {
		o.apiEnforcer = apiEnforcer
	}
}

// MakeResponseEncoder generates an nject Provider to encode API responses.
//
// The generated provider is a wrapper that invokes the rest of the
// handler injection chain and expect to receive as return values
// an Response and and error.  If the error is not nil, then the response
// becomes the error.
//
// If more than one encoder is configurured, then MakeResponseEncoder will default to
// the first one specified in its functional arguments.
func MakeResponseEncoder(
	name string,
	encoderFuncArgs ...ResponseEncoderFuncArg,
) nject.Provider {
	o := encoderOptions{
		errorTransformer: func(_ error) (interface{}, bool) { return nil, false },
		encoders:         make(map[string]specificEncoder),
	}
	for _, fa := range encoderFuncArgs {
		fa(&o)
	}
	if o.defaultEncoder == "" {
		// oops, the user should have done something!
		WithEncoder("application/json", json.Marshal)(&o)
	}
	return nject.Provide("marshal-"+name,
		func(
			inner func() (Response, error),
			w *DeferredWriter,
			log BasicLogger,
			r *http.Request,
		) {
			model, err := inner()
			if w.Done() {
				return
			}
			contentType := httputil.NegotiateContentType(r, o.contentOffers, o.defaultEncoder)
			encoder := o.encoders[contentType]
			var code int
			var enc []byte

			// handleError will always set enc
			var handleError func(recurseOkay bool)
			handleError = func(recurseOkay bool) {
				code = GetReturnCode(err)
				et := encoder.errorTransformer
				if et == nil {
					et = o.errorTransformer
				}
				logDetails := map[string]interface{}{
					"httpCode": code,
					"error":    err.Error(),
					"method":   r.Method,
					"uri":      r.URL.String(),
				}
				if code < 500 {
					log.Warn("returning user error", logDetails)
				} else {
					log.Error("returning server error", logDetails)
				}
				if rm, ok := et(err); ok {
					enc, err = encoder.encode(rm)
					if err != nil {
						err = errors.Wrapf(err, "encode %s response", contentType)
						if recurseOkay {
							handleError(false)
						} else {
							enc = []byte(err.Error())
						}
					}
				} else {
					enc = []byte(err.Error())
				}
			}
			if err != nil {
				handleError(true)
			}

			if len(enc) == 0 {
				enc, err = encoder.encode(model)
				if err != nil {
					handleError(true)
				}
			}

			err = encoder.apiEnforcer(code, enc, w.Header(), r)
			if err != nil {
				handleError(true)
			}
			w.WriteHeader(code)
			_, err = w.Write(enc)
			e2 := w.Flush()
			if err == nil {
				err = e2
			}
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
var Nil204 = nject.Desired(nject.Provide("nil-204", nil204))

func nil204(inner func() (Response, error), w *DeferredWriter) {
	model, err := inner()
	if w.Done() {
		return
	}
	if err == nil && model == nil {
		w.WriteHeader(204)
		_ = w.Flush()
	}
}
