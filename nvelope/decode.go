package nvelope

import (
	"encoding"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"github.com/muir/nject/nject"
	"github.com/pkg/errors"
)

// Body is a type provideded by ReadBody: it is a []byte
// with the request body pre-read.
type Body []byte

// ReadBody is a provider that reads the input body from
// an http.Request and provides it in the Body type.
var ReadBody = nject.Provide("read-body", readBody)

func readBody(r *http.Request) (Body, nject.TerminalError) {
	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	return Body(body), err
}

// DecodeJSON is is a pre-defined special nject.Provider
// created with GenerateDecoder for decoding JSON requests.
var DecodeJSON = GenerateDecoder(
	WithDecoder("application/json", json.Unmarshal),
	WithDefaultDecoder(json.Unmarshal),
)

type Decoder func([]byte, interface{}) error

type eigo struct {
	tag             string
	decoders        map[string]Decoder
	modelValidators []func(interface{}) error
	methodIfPresent []string
}

type DecodeInputsGeneratorOpt func(*eigo)

// WithDecoder maps conent types (eg "application/json") to
// decode functions (eg json.Unmarshal).  If a Content-Type header
// is used in the requet, then the value of that header will be
// used to pick a decoder.
func WithDecoder(contentType string, decoder Decoder) DecodeInputsGeneratorOpt {
	return func(o *eigo) {
		o.decoders[contentType] = decoder
	}
}

// WithDefaultDecoder specifies which model decoder to use when
// no "Content-Type" header was sent.
func WithDefaultDecoder(decoder Decoder) DecodeInputsGeneratorOpt {
	return func(o *eigo) {
		o.decoders[""] = decoder
	}
}

/* TODO
func WithModelValidator(f func(interface{}) error) DecodeInputsGeneratorOpt {
	return func(o *eigo) {
		o.modelValidators = append(o.modelValidators, f)
	}
}
*/

/* TODO
func CallModelMethodIfPresent(method string) DecodeInputsGeneratorOpt {
	return func(o *eigo) {
		o.methodIfPresent = append(o.methodIfPresent, method)
	}
}
*/

// WithTag overrides the tag for specifying fields to be filled
// from the http request.  The default is "nvelope"
func WithTag(tag string) DecodeInputsGeneratorOpt {
	return func(o *eigo) {
		o.tag = tag
	}
}

// TODO: Does this work?
// This model can be defined right in the function though:
//
//  func HandleEndpoint(
//    inputs struct {
//      EndpointRequestModel `nvelope:model`
//    }) (nvelope.Any, error) {
//      ...
//  }

// TODO: handle multipart form uploads

// GenerateDecoder injects a special provider that uses
// nject.GenerateFromInjectionChain to examine the injection
// chain to see if there are any models that are used but
// never provided.  If so, it looks at the struct tags in
// the models to see if they are tagged for filling with
// the decoder.  If so, the provider is created that injects
// the missing model into the dependency chain.  The intended
// use for this is to have an endpoint handler receive the
// deocded request body.
//
// Major warning: the endpoint handler must receive the request
// model as a field inside a model, not as a standalone model.
//
// The following tags are recognized:
//
// `nvelope:"model"` causes the POST or PUT body to be decoded
// using a decoder like json.Unmarshal.
//
// `nvelope:"path,name=xxx"` causes part of the URL path to
// be extracted and written to the tagged field.
//
// `nvelope:"query,name=xxx"` causes the named URL query
// parameters to be extracted and written to the tagged field.
//
// `nvelope:"header,name=xxx"` causes the named HTTP header
// to be extracted and written to the tagged field.
//
// GenerateDecoder depends upon and uses Gorilla mux.
func GenerateDecoder(
	genOpts ...DecodeInputsGeneratorOpt,
) interface{} {
	options := eigo{
		tag:      "nvelope",
		decoders: make(map[string]Decoder),
	}
	for _, opt := range genOpts {
		opt(&options)
	}
	return nject.GenerateFromInjectionChain(func(before nject.Collection, after nject.Collection) (nject.Provider, error) {
		full := before.Append("after", after)
		missingInputs, _ := full.DownFlows()
		var providers []interface{}
		for _, missingType := range missingInputs {
			returnType := missingType
			var nonPointer reflect.Type
			var returnAddress bool
			switch missingType.Kind() {
			case reflect.Struct:
				nonPointer = returnType
			case reflect.Ptr:
				returnAddress = true
				e := returnType.Elem()
				if e.Kind() != reflect.Struct {
					continue
				}
				nonPointer = e
			default:
				continue
			}
			var varsFillers []func(model reflect.Value, vars map[string]string) error
			var headerFillers []func(model reflect.Value, header http.Header) error
			var queryFillers []func(model reflect.Value, query url.Values) error
			var bodyFillers []func(model reflect.Value, body []byte, r *http.Request) error
			var returnError error
			walkStructElements(nonPointer, func(field reflect.StructField) bool {
				tag, ok := field.Tag.Lookup(options.tag)
				if !ok {
					return true
				}
				base, kv := parseTag(tag)
				if base == "model" {
					bodyFillers = append(bodyFillers,
						func(model reflect.Value, body []byte, r *http.Request) error {
							f := model.FieldByIndex(field.Index)
							ct := r.Header.Get("Content-Type")
							exactDecoder, ok := options.decoders[ct]
							if !ok {
								return errors.Errorf("No body decoder for content type %s", ct)
							}
							return exactDecoder(body, f.Interface())
						})
					return false
				}

				name := field.Name // not used by model, but used by the rest
				if n, ok := kv["name"]; ok {
					name = n
				}
				var unpack func(from string, target reflect.Value, value string) error
				var err error
				if field.Type.Kind() == reflect.Slice && (base == "header" || base == "query") {
					unpack, err = getUnpacker(field.Type.Elem(), field.Name, name)
				} else {
					unpack, err = getUnpacker(field.Type, field.Name, name)
				}
				if err != nil {
					returnError = err
					return false
				}
				switch base {
				case "path":
					varsFillers = append(varsFillers, func(model reflect.Value, vars map[string]string) error {
						f := model.FieldByIndex(field.Index)
						return unpack("path", f, vars[name])
					})
				case "header":
					if field.Type.Kind() == reflect.Slice {
						headerFillers = append(headerFillers, func(model reflect.Value, header http.Header) error {
							f := model.FieldByIndex(field.Index)
							return multiUnpack("header", f, unpack, header[name])
						})
					} else {
						headerFillers = append(headerFillers, func(model reflect.Value, header http.Header) error {
							f := model.FieldByIndex(field.Index)
							return unpack("header", f, header.Get(name))
						})
					}
				case "query":
					if field.Type.Kind() == reflect.Slice {
						queryFillers = append(queryFillers, func(model reflect.Value, query url.Values) error {
							f := model.FieldByIndex(field.Index)
							return multiUnpack("query", f, unpack, query[name])
						})
					} else {
						queryFillers = append(queryFillers, func(model reflect.Value, query url.Values) error {
							f := model.FieldByIndex(field.Index)
							return unpack("query", f, query.Get(name))
						})
					}
				default:
					returnError = errors.Errorf(
						"unknown tag %s value in %s struct: %s",
						options.tag, nonPointer, base)
					return false
				}
				return true
			})
			if returnError != nil {
				return nil, returnError
			}

			if len(varsFillers) == 0 &&
				len(headerFillers) == 0 &&
				len(queryFillers) == 0 &&
				len(bodyFillers) == 0 {
				continue
			}

			inputs := []reflect.Type{httpRequestType}
			if len(bodyFillers) != 0 {
				inputs = append(inputs, bodyType)
			}
			outputs := []reflect.Type{returnType, terminalErrorType}

			reflective := nject.MakeReflective(inputs, outputs, func(in []reflect.Value) []reflect.Value {
				fmt.Println("XXX begin decode")
				r := in[0].Interface().(*http.Request)
				mp := reflect.New(nonPointer)
				model := mp.Elem()
				var err error
				setError := func(e error) {
					if err == nil && e != nil {
						err = e
					}
				}
				if len(bodyFillers) != 0 {
					body := []byte(in[1].Interface().(Body))
					for _, bf := range bodyFillers {
						setError(bf(model, body, r))
					}
				}
				if len(varsFillers) != 0 {
					vars := mux.Vars(r)
					for _, vf := range varsFillers {
						setError(vf(model, vars))
					}
				}
				for _, hf := range headerFillers {
					setError(hf(model, r.Header))
				}
				if len(queryFillers) != 0 {
					vals := r.URL.Query()
					for _, qf := range queryFillers {
						setError(qf(model, vals))
					}
				}
				ev := reflect.ValueOf(err)
				fmt.Println("XXX end decode", err)
				if returnAddress {
					return []reflect.Value{mp, ev}
				} else {
					return []reflect.Value{model, ev}
				}
			})
			providers = append(providers, nject.Provide("create "+nonPointer.String(), reflective))
		}
		return nject.Sequence("fill functions from request", providers...), nil
	})
}

func multiUnpack(
	from string, f reflect.Value,
	singleUnpack func(from string, target reflect.Value, value string) error,
	values []string,
) error {
	a := reflect.MakeSlice(f.Type(), len(values), len(values))
	for i, value := range values {
		err := singleUnpack(from, a.Index(i), value)
		if err != nil {
			return err
		}
	}
	return nil
}

func getUnpacker(fieldType reflect.Type, fieldName string, name string,
) (func(from string, target reflect.Value, value string) error, error) {
	if fieldType.AssignableTo(textUnmarshallerType) {
		return func(from string, target reflect.Value, value string) error {
			return errors.Wrapf(
				target.Interface().(encoding.TextUnmarshaler).UnmarshalText([]byte(value)),
				"decode %s %s", from, name)
		}, nil
	}
	if reflect.PtrTo(fieldType).AssignableTo(textUnmarshallerType) {
		return func(from string, target reflect.Value, value string) error {
			return errors.Wrapf(
				target.Addr().Interface().(encoding.TextUnmarshaler).UnmarshalText([]byte(value)),
				"decode %s %s", from, name)
		}, nil
	}
	switch fieldType.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return func(from string, target reflect.Value, value string) error {
			i, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return errors.Wrapf(err, "decode %s %s", from, name)
			}
			target.SetInt(i)
			return nil
		}, nil
	case reflect.Uint, reflect.Uintptr, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return func(from string, target reflect.Value, value string) error {
			i, err := strconv.ParseUint(value, 10, 64)
			if err != nil {
				return errors.Wrapf(err, "decode %s %s", from, name)
			}
			target.SetUint(i)
			return nil
		}, nil
	case reflect.Float32, reflect.Float64:
		return func(from string, target reflect.Value, value string) error {
			f, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return errors.Wrapf(err, "decode %s %s", from, name)
			}
			target.SetFloat(f)
			return nil
		}, nil
	case reflect.String:
		return func(_ string, target reflect.Value, value string) error {
			target.SetString(value)
			return nil
		}, nil
	// TODO: case reflect.Slice:
	// TODO: case reflect.Array:
	default:
		return nil, errors.Errorf(
			"Cannot decode into %s, %s does not implement UnmarshalText",
			fieldName, fieldType)
	}
}

var httpRequestType = reflect.TypeOf(&http.Request{})
var bodyType = reflect.TypeOf(Body{})
var textUnmarshallerType = reflect.TypeOf((*encoding.TextUnmarshaler)(nil)).Elem()
var terminalErrorType = reflect.TypeOf((*nject.TerminalError)(nil)).Elem()
var emptyInterfaceType = reflect.TypeOf((*interface{})(nil)).Elem()
var errorType = reflect.TypeOf((*error)(nil)).Elem()

// The return value from f only matters when the type of the field is a struct.  In
// that case, a false value prevents recursion.
func walkStructElements(t reflect.Type, f func(reflect.StructField) bool) {
	if t.Kind() == reflect.Struct {
		doWalkStructElements(t, []int{}, f)
	}
	if t.Kind() == reflect.Ptr && t.Elem().Kind() == reflect.Struct {
		doWalkStructElements(t.Elem(), []int{}, f)
	}
	return
}

func doWalkStructElements(t reflect.Type, path []int, f func(reflect.StructField) bool) {
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		np := copyIntSlice(path)
		np = append(np, field.Index...)
		field.Index = np
		if f(field) && field.Type.Kind() == reflect.Struct {
			doWalkStructElements(field.Type, np, f)
		}
	}
}

func copyIntSlice(in []int) []int {
	c := make([]int, len(in), len(in)+1)
	copy(c, in)
	return c
}

func parseTag(s string) (string, map[string]string) {
	a := strings.Split(s, ",")
	if len(a) == 1 {
		return s, nil
	}
	kv := make(map[string]string)
	for _, v := range a[1:] {
		kvs := strings.SplitN(v, ",", 2)
		k := kvs[0]
		if len(kvs) == 2 {
			kv[k] = kvs[1]
		} else {
			kv[k] = ""
		}
	}
	return a[0], kv
}
