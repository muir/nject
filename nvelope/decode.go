package nvelope

import (
	"bytes"
	"encoding"
	"encoding/json"
	"encoding/xml"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"github.com/muir/nject/nject"
	"github.com/muir/reflectutils"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

// Body is a type provideded by ReadBody: it is a []byte
// with the request body pre-read.
type Body []byte

// ReadBody is a provider that reads the input body from
// an http.Request and provides it in the Body type.
var ReadBody = nject.Provide("read-body", readBody)

func readBody(r *http.Request) (Body, nject.TerminalError) {
	// nolint:errcheck
	defer r.Body.Close()
	body, err := ioutil.ReadAll(r.Body)
	r.Body = ioutil.NopCloser(bytes.NewReader(body))
	return Body(body), err
}

// DecodeJSON is is a pre-defined special nject.Provider
// created with GenerateDecoder for decoding JSON requests.
var DecodeJSON = GenerateDecoder(
	WithDecoder("application/json", json.Unmarshal),
	WithDefaultContentType("application/json"),
)

// DecodeXML is is a pre-defined special nject.Provider
// created with GenerateDecoder for decoding XML requests.
var DecodeXML = GenerateDecoder(
	WithDecoder("application/xml", xml.Unmarshal),
	WithDefaultContentType("application/xml"),
)

// Decoder is the signature for decoders: take bytes and
// a pointer to something and deserialize it.
type Decoder func([]byte, interface{}) error

type eigo struct {
	tag                string
	decoders           map[string]Decoder
	defaultContentType string
}

// DecodeInputsGeneratorOpt are functional arguments for
// GenerateDecoder
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

// WithDefaultContentType specifies which model decoder to use when
// no "Content-Type" header was sent.
func WithDefaultContentType(contentType string) DecodeInputsGeneratorOpt {
	return func(o *eigo) {
		o.defaultContentType = contentType
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
// `nvelope:"cookie,name=xxx"` cause the named HTTP cookie to be
// extracted and writted to the tagged field.
//
// Path, query, header, and cookie support options described
// in https://swagger.io/docs/specification/serialization/ for
// controlling how to serialize.  The following are supported
// as appropriate.
//
//	explode=true			# default for query
//	explode=false			# default for path, header
//	delimiter=comma			# default
//	delimiter=space			# query parameters only
//	delimiter=pipe			# query parameters only
//	allowReserved=false		# default
//	allowReserved=true		# query parameters only
//	form=false			# default
//	form=true			# cookies only
//	content=application/json	# specifies that the value should be decoded with JSON
//	content=application/xml		# specifies that the value should be decoded with XML
//
// "style=label" and "style=matrix" are NOT yet supported for path parameters.
// "explode=false" is not supported for query parameters mapping to structs.
// "deepObject=true" is not supported
//
// Generally setting "content" to something should be paired with "explode=false"
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
			// nolint:exhaustive
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
			var cookieFillers []func(model reflect.Value, r *http.Request) error
			var bodyFillers []func(model reflect.Value, body []byte, r *http.Request) error
			var returnError error
			reflectutils.WalkStructElements(nonPointer, func(field reflect.StructField) bool {
				tag, ok := field.Tag.Lookup(options.tag)
				if !ok {
					return true
				}
				base, tags, err := parseTag(tag)
				if err != nil {
					returnError = err
					return false
				}
				if base == "model" {
					bodyFillers = append(bodyFillers,
						func(model reflect.Value, body []byte, r *http.Request) error {
							f := model.FieldByIndex(field.Index)
							ct := r.Header.Get("Content-Type")
							if ct == "" {
								ct = options.defaultContentType
							}
							exactDecoder, ok := options.decoders[ct]
							if !ok {
								return errors.Errorf("No body decoder for content type %s", ct)
							}
							err := exactDecoder(body, f.Addr().Interface())
							return errors.Wrapf(err, "Could not decode %s into %s", ct, field.Type)
						})
					return false
				}

				name := field.Name // not used by model, but used by the rest
				if tags.name != "" {
					name = tags.name
				}
				unpackerType := field.Type
				unpack, multiUnpack, err := getUnpacker(unpackerType, field.Name, name, base, tags, options.decoders)
				if err != nil {
					returnError = err
					return false
				}
				switch base {
				case "path":
					varsFillers = append(varsFillers, func(model reflect.Value, vars map[string]string) error {
						f := model.FieldByIndex(field.Index)
						return errors.Wrapf(
							unpack("path", f, vars[name]),
							"path element %s into field %s",
							name, field.Name)
					})
				case "header":
					if multiUnpack != nil {
						headerFillers = append(headerFillers, func(model reflect.Value, header http.Header) error {
							f := model.FieldByIndex(field.Index)
							return errors.Wrapf(
								multiUnpack("header", f, header[name]),
								"header %s into field %s",
								name, field.Name)
						})
					} else {
						headerFillers = append(headerFillers, func(model reflect.Value, header http.Header) error {
							f := model.FieldByIndex(field.Index)
							return errors.Wrapf(
								unpack("header", f, header.Get(name)),
								"header %s into field %s",
								name, field.Name)
						})
					}
				case "query":
					if multiUnpack != nil {
						queryFillers = append(queryFillers, func(model reflect.Value, query url.Values) error {
							f := model.FieldByIndex(field.Index)
							return errors.Wrapf(
								multiUnpack("query", f, query[name]),
								"query parameter %s into field %s",
								name, field.Name)
						})
					} else {
						queryFillers = append(queryFillers, func(model reflect.Value, query url.Values) error {
							f := model.FieldByIndex(field.Index)
							return errors.Wrapf(
								unpack("query", f, query.Get(name)),
								"query parameter %s into field %s",
								name, field.Name)
						})
					}
				case "cookie":
					cookieFillers = append(cookieFillers, func(model reflect.Value, r *http.Request) error {
						f := model.FieldByIndex(field.Index)
						cookie, err := r.Cookie(name)
						if err != nil {
							if err == http.ErrNoCookie {
								return nil
							}
							return errors.Wrapf(err, "cookie parameter %s into field %s", name, field.Name)
						}
						return errors.Wrapf(
							unpack("cookie", f, cookie.Value),
							"cookie parameter %s into field %s",
							name, field.Name)
					})
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
				// nolint:errcheck
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
				for _, cf := range cookieFillers {
					setError(cf(model, r))
				}
				var ev reflect.Value
				if err == nil {
					ev = reflect.Zero(errorType)
				} else {
					ev = reflect.ValueOf(errors.Wrapf(ReturnCode(err, 400), "%s model", returnType))
				}
				if returnAddress {
					return []reflect.Value{mp, ev}
				}
				return []reflect.Value{model, ev}
			})
			providers = append(providers, nject.Provide("create "+nonPointer.String(), reflective))
		}
		return nject.Sequence("fill functions from request", providers...), nil
	})
}

// generateStructUnpacker generates a function to deal with filling a struct from
// an array of key, value pairs.
func generateStructUnpacker(
	fieldType reflect.Type,
	tagName string,
) (
	func(from string, f reflect.Value, values []string) error,
	error,
) {
	type fillTarget struct {
		field  reflect.StructField
		filler func(from string, target reflect.Value, value string) error
	}
	targets := make(map[string]fillTarget)
	var anyErr error
	reflectutils.WalkStructElements(fieldType, func(field reflect.StructField) bool {
		tag, ok := field.Tag.Lookup(tagName)
		if !ok {
			return true
		}
		name, tags, err := parseTag(tag)
		if err != nil {
			anyErr = errors.Wrap(err, field.Name)
			return false
		}
		if _, ok := targets[name]; ok {
			anyErr = errors.Errorf("Only one field can be filled with the same name.  '%s' is duplicated.  One example is %s",
				name, field.Name)
			return false
		}
		tags.explode = false
		unpacker, _, err := getUnpacker(field.Type, field.Name, name, "XXX", tags, nil)
		if err != nil {
			anyErr = errors.Wrap(err, field.Name)
			return false
		}
		targets[name] = fillTarget{
			field:  field,
			filler: unpacker,
		}
		return true
	})
	if anyErr != nil {
		return nil, anyErr
	}
	return func(from string, model reflect.Value, values []string) error {
		for i := 0; i < len(values); i += 2 {
			keyString := values[i]
			var valueString string
			if i+1 < len(values) {
				valueString = values[i+1]
			}
			target, ok := targets[keyString]
			if !ok {
				return errors.Errorf("No struct member to receive key '%s'", keyString)
			}
			f := model.FieldByIndex(target.field.Index)
			err := target.filler(from, f, valueString)
			if err != nil {
				return errors.Wrap(err, target.field.Name)
			}
		}
		return nil
	}, nil
}

func mapUnpack(
	from string, f reflect.Value,
	keyUnpack func(from string, target reflect.Value, value string) error,
	valueUnpack func(from string, target reflect.Value, value string) error,
	values []string,
) error {
	m := reflect.MakeMap(f.Type())
	for i := 0; i < len(values); i += 2 {
		keyString := values[i]
		var valueString string
		if i+1 < len(values) {
			valueString = values[i+1]
		}
		keyPointer := reflect.New(f.Type().Key())
		err := keyUnpack(from, keyPointer, keyString)
		if err != nil {
			return err
		}
		valuePointer := reflect.New(f.Type().Elem())
		err = valueUnpack(from, valuePointer, valueString)
		if err != nil {
			return err
		}
		m.SetMapIndex(reflect.Indirect(keyPointer), reflect.Indirect(valuePointer))
	}
	f.Set(m)
	return nil
}

func arrayUnpack(
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
	f.Set(a)
	return nil
}

type parameterContext int

const (
	pathParemeter parameterContext = iota
	queryParameter
	headerParameter
	cookieParameter
)

// getUnpacker is used for unpacking headers, query parameters, and path elements
func getUnpacker(
	fieldType reflect.Type,
	fieldName string,
	name string,
	base string, // "path", "query", etc.
	tags tags,
	decoders map[string]Decoder,
) (
	func(from string, target reflect.Value, value string) error,
	func(from string, target reflect.Value, values []string) error,
	error) {
	if fieldType.AssignableTo(textUnmarshallerType) {
		return func(from string, target reflect.Value, value string) error {
			return errors.Wrapf(
				target.Interface().(encoding.TextUnmarshaler).UnmarshalText([]byte(value)),
				"decode %s %s", from, name)
		}, nil, nil // XXX?
	}
	if reflect.PtrTo(fieldType).AssignableTo(textUnmarshallerType) {
		return func(from string, target reflect.Value, value string) error {
			return errors.Wrapf(
				target.Addr().Interface().(encoding.TextUnmarshaler).UnmarshalText([]byte(value)),
				"decode %s %s", from, name)
		}, nil, nil // XXX?
	}
	if tags.content != "" {
		return contentUnpacker(fieldType, fieldName, name, base, tags, decoders)
	}

	switch fieldType.Kind() {
	case reflect.Ptr:
		vu, mu, err := getUnpacker(fieldType.Elem(), fieldName, name, base, tags, decoders)
		if err != nil {
			return nil, nil, err
		}
		if mu != nil {
			return nil, func(from string, target reflect.Value, values []string) error {
				p := reflect.New(fieldType.Elem())
				target.Set(p)
				return mu(from, target.Elem(), values)
			}, nil
		}
		return func(from string, target reflect.Value, value string) error {
			p := reflect.New(fieldType.Elem())
			target.Set(p)
			return vu(from, target.Elem(), value)
		}, nil, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return func(from string, target reflect.Value, value string) error {
			i, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return errors.Wrapf(err, "decode %s %s", from, name)
			}
			target.SetInt(i)
			return nil
		}, nil, nil
	case reflect.Uint, reflect.Uintptr, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return func(from string, target reflect.Value, value string) error {
			i, err := strconv.ParseUint(value, 10, 64)
			if err != nil {
				return errors.Wrapf(err, "decode %s %s", from, name)
			}
			target.SetUint(i)
			return nil
		}, nil, nil
	case reflect.Float32, reflect.Float64:
		return func(from string, target reflect.Value, value string) error {
			f, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return errors.Wrapf(err, "decode %s %s", from, name)
			}
			target.SetFloat(f)
			return nil
		}, nil, nil
	case reflect.String:
		return func(_ string, target reflect.Value, value string) error {
			target.SetString(value)
			return nil
		}, nil, nil
	case reflect.Complex64, reflect.Complex128:
		return func(from string, target reflect.Value, value string) error {
			c, err := strconv.ParseComplex(value, 128)
			if err != nil {
				return errors.Wrapf(err, "decode %s %s", from, name)
			}
			target.SetComplex(c)
			return nil
		}, nil, nil
	case reflect.Bool:
		return func(from string, target reflect.Value, value string) error {
			b, err := strconv.ParseBool(value)
			if err != nil {
				return errors.Wrapf(err, "decode %s %s", from, name)
			}
			target.SetBool(b)
			return nil
		}, nil, nil

	case reflect.Slice:
		switch base {
		case "cookie", "path":
			if tags.delimiter != "," {
				return nil, nil, errors.New("delimiter setting is only allowed for 'query' parameters")
			}
		}
		singleUnpack, _, err := getUnpacker(fieldType.Elem(), fieldName, name, base, tags.WithoutExplode(), decoders)
		if err != nil {
			return nil, nil, err
		}
		switch base {
		case "query", "header":
			if tags.explode {
				return nil, func(from string, target reflect.Value, values []string) error {
					return arrayUnpack(from, target, singleUnpack, values)
				}, nil
			}
		}
		return func(from string, target reflect.Value, value string) error {
			values := strings.Split(value, tags.delimiter)
			return arrayUnpack(from, target, singleUnpack, values)
		}, nil, nil

	case reflect.Struct:
		structUnpacker, err := generateStructUnpacker(fieldType, fieldName)
		if err != nil {
			return nil, nil, err
		}
		switch base {
		case "query", "header":
			if tags.explode {
				return nil, func(from string, target reflect.Value, values []string) error {
					return structUnpacker(from, target, resplitOnEquals(values))
				}, nil
			}
		}
		return func(from string, target reflect.Value, value string) error {
			values := strings.Split(value, tags.delimiter)
			return structUnpacker(from, target, values)
		}, nil, nil

	case reflect.Map:
		switch base {
		case "cookie", "path":
			if tags.delimiter != "," {
				return nil, nil, errors.New("delimiter setting is only allowed for 'query' parameters")
			}
		}
		keyUnpack, _, err := getUnpacker(fieldType.Key(), fieldName, name, base, tags.WithoutExplode(), decoders)
		if err != nil {
			return nil, nil, err
		}
		elementUnpack, _, err := getUnpacker(fieldType.Elem(), fieldName, name, base, tags.WithoutExplode(), decoders)
		if err != nil {
			return nil, nil, err
		}
		switch base {
		case "query", "header":
			if tags.explode {
				return nil, func(from string, target reflect.Value, values []string) error {
					return mapUnpack(from, target, keyUnpack, elementUnpack, resplitOnEquals(values))
				}, nil
			}
		}
		return func(from string, target reflect.Value, value string) error {
			values := strings.Split(value, tags.delimiter)
			return arrayUnpack(from, target, elementUnpack, resplitOnEquals(values))
		}, nil, nil

	case reflect.Array:
		// TODO: handle arrays
		fallthrough
	case reflect.Chan, reflect.Interface, reflect.UnsafePointer, reflect.Func, reflect.Invalid:
		fallthrough
	default:
		return nil, nil, errors.Errorf(
			"Cannot decode into %s, %s does not implement UnmarshalText",
			fieldName, fieldType)
	}
}

// contentUnpacker generates an unpacker to use when something has
// been tagged "content=application/json" or such.  We bypass our
// regular unpackers and instead use a regular decoder.  The interesting
// case is where this is combined with "explode=true" because then
// we have to decode many times
func contentUnpacker(
	fieldType reflect.Type,
	fieldName string,
	name string,
	base string, // "path", "query", etc.
	tags tags,
	decoders map[string]Decoder,
) (
	func(from string, target reflect.Value, value string) error,
	func(from string, target reflect.Value, values []string) error,
	error) {

	decoder, ok := decoders[tags.content]
	if !ok {
		// tags.content can provide access to decoders beyond what
		// is specified for GenerateDecoder
		switch tags.content {
		case "application/json":
			decoder = json.Unmarshal
		case "application/xml":
			decoder = xml.Unmarshal
		case "application/yaml":
			decoder = yaml.Unmarshal
		default:
			errors.Errorf("No decoder provided for content type '%s'", tags.content)
		}
	}
	kind := fieldType.Kind()
	if tags.explode &&
		(base == "query" || base == "header") &&
		(kind == reflect.Map || kind == reflect.Slice) {
		valueUnpack, _, err := getUnpacker(fieldType.Elem(), fieldName, name, base, tags.WithoutExplode(), decoders)
		if err != nil {
			return nil, nil, err
		}
		if kind == reflect.Slice {
			return nil, func(from string, target reflect.Value, values []string) error {
				a := reflect.MakeSlice(target.Type(), len(values), len(values))
				for i, valueString := range values {
					err := valueUnpack(from, a.Index(i), valueString)
					if err != nil {
						return err
					}
				}
				target.Set(a)
				return nil
			}, nil
		}
		keyUnpack, _, err := getUnpacker(fieldType.Key(), fieldName, name, base, tags.WithoutExplode().WithoutContent(), decoders)
		return nil, func(from string, target reflect.Value, values []string) error {
			m := reflect.MakeMap(target.Type())
			for _, pair := range values {
				kv := strings.SplitN(pair, "=", 2)
				keyString := kv[0]
				var valueString string
				if len(kv) == 2 {
					valueString = kv[1]
				}
				keyPointer := reflect.New(fieldType.Key())
				err := keyUnpack(from, keyPointer, keyString)
				if err != nil {
					return err
				}
				valuePointer := reflect.New(fieldType.Elem())
				err = valueUnpack(from, valuePointer, valueString)
				if err != nil {
					return err
				}
				m.SetMapIndex(reflect.Indirect(keyPointer), reflect.Indirect(valuePointer))
			}
			target.Set(m)
			return nil
		}, nil
	}

	return func(from string, target reflect.Value, value string) error {
		i := target.Addr().Interface()
		err := decoder([]byte(value), i)
		return errors.Wrap(err, fieldName)
	}, nil, nil
}

var (
	httpRequestType      = reflect.TypeOf(&http.Request{})
	bodyType             = reflect.TypeOf(Body{})
	textUnmarshallerType = reflect.TypeOf((*encoding.TextUnmarshaler)(nil)).Elem()
	terminalErrorType    = reflect.TypeOf((*nject.TerminalError)(nil)).Elem()
	errorType            = reflect.TypeOf((*error)(nil)).Elem()
)

var delimiters = map[string]string{
	"comma": ",",
	"pipe":  "|",
	"space": " ",
}

type tags struct {
	name          string
	explode       bool
	delimiter     string
	allowReserved bool
	content       string
}

func (tags tags) WithoutExplode() tags {
	tags.explode = false
	return tags
}

func (tags tags) WithoutContent() tags {
	tags.content = ""
	return tags
}

func parseTag(s string) (string, tags, error) {
	a := strings.Split(s, ",")
	var tags tags
	if len(a) == 0 {
		return "", tags, errors.New("must specify the source of the data ('path', 'query', etc)")
	}
	tags.delimiter = ","
	switch a[0] {
	case "path":
	case "query":
		tags.explode = true
	case "header":
	case "cookie":
	case "model":
	default:
		return "", tags, errors.Errorf("'%s' is not a valid source of the data use ('model', 'path', 'query', etc)", a[0])
	}
	for _, v := range a[1:] {
		kvs := strings.SplitN(v, "=", 2)
		k := kvs[0]
		var val string
		if len(kvs) == 2 {
			val = kvs[1]
		}
		var err error
		switch k {
		case "name":
			tags.name = val
		case "explode":
			tags.explode, err = strconv.ParseBool(val)
		case "delimiter":
			var ok bool
			tags.delimiter, ok = delimiters[val]
			if !ok {
				err = errors.Errorf("Invalid delimiter value (must be 'comma', 'space', or 'pipe')")
			}
		case "allowReserved":
			tags.allowReserved, err = strconv.ParseBool(val)
		case "content":
			tags.content = val
		}
		if err != nil {
			return "", tags, errors.Wrap(err, k)
		}
	}
	return a[0], tags, nil
}

func resplitOnEquals(values []string) []string {
	nv := make([]string, len(values)*2)
	for i, v := range values {
		a := strings.SplitN(v, "=", 2)
		nv[i*2] = a[0]
		if len(a) == 2 {
			nv[i*2+1] = a[1]
		}
	}
	return nv
}
