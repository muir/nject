package nvelope

import (
	"bytes"
	"encoding"
	"encoding/json"
	"encoding/xml"
	"io/ioutil"
	"net/http"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/muir/nject/nject"
	"github.com/muir/reflectutils"

	"github.com/gorilla/mux"
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
	tag                          string
	decoders                     map[string]Decoder
	defaultContentType           string
	rejectUnknownQueryParameters bool
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

// RejectUnknownQueryParameters true indicates that if there are any
// query parameters supplied that were not expected, the request should
// be rejected with a 400 response code.  This parameter also controls
// what happens if there an embedded object is filled and there is no
// object key corresponding to the request parameter.
//
// This does not apply to query parameters with content=application/json
// decodings.  If you want to disallow unknown tags for content= decodings,
// define a custom decoder.
func RejectUnknownQueryParameters(b bool) DecodeInputsGeneratorOpt {
	return func(o *eigo) {
		o.rejectUnknownQueryParameters = b
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

var deepObjectRE = regexp.MustCompile(`^([^\[]+)\[([^\]]+)\]$`) // id[name]

// TODO: handle multipart form uploads

// GenerateDecoder injects a special provider that uses
// nject.GenerateFromInjectionChain to examine the injection
// chain to see if there are any models that are used but
// never provided.  If so, it looks at the struct tags in
// the models to see if they are tagged for filling with
// the decoder.  If so, a provider is created that injects
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
//	explode=true			# default for query, header
//	explode=false			# default for path
//	delimiter=comma			# default
//	delimiter=space			# query parameters only
//	delimiter=pipe			# query parameters only
//	allowReserved=false		# default
//	allowReserved=true		# query parameters only
//	form=false			# default
//	form=true			# cookies only
//	content=application/json	# specifies that the value should be decoded with JSON
//	content=application/xml		# specifies that the value should be decoded with XML
//	deepObject=false		# default
//	deepObject=true			# required for query object
//
// "style=label" and "style=matrix" are NOT yet supported for path parameters.
//
// For query parameters filling maps and structs, the only the following
// combinations are supported:
//
//	deepObject=true
//	deepObject=false,explode=false
//
// When filling embedded structs from query, or header, parameters,
// using explode=false or deepObject=true, tagging struct members is
// optional.  Tag them with their name or with "-" if you do not want
// them filled.
//
// 	type Fillme struct {
//		Embedded struct {
//			IntValue    int                     // will get filled by key "IntValue"
//			FloatValue  float64 `nvelope:"-"`   // will not get filled
//			StringValue string  `nvelope:"bob"` // will get filled by key "bob"
//		} `nvelope:"query,name=embedded,explode=false"`
//	}
//
// "deepObject=true" is only supported for maps and structs and only for query parameters.
//
// Generally setting "content" to something should be paired with "explode=false"
//
// GenerateDecoder depends upon and uses Gorilla mux.
//
// GenerateDecoder uses https://pkg.go.dev/github.com/muir/reflectutils#MakeStringSetter to
// unpack strings into struct fields.  That provides support for time.Duration and anything
// that implements encoding.TextUnmarshaler or flag.Value.  Additional custom decoders can
// be registered with https://pkg.go.dev/github.com/muir/reflectutils#RegisterStringSetter .
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
			var cookieFillers []func(model reflect.Value, r *http.Request) error
			var bodyFillers []func(model reflect.Value, body []byte, r *http.Request) error
			queryFillers := make(map[string]func(reflect.Value, []string) error)
			deepObjectFillers := make(map[string]func(reflect.Value, map[string][]string) error)
			var returnError error
			reflectutils.WalkStructElements(nonPointer, func(field reflect.StructField) bool {
				tag, ok := field.Tag.Lookup(options.tag)
				if !ok {
					return true
				}
				base, tags, err := parseTag(tag, true)
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
							// nolint:govet
							err := exactDecoder(body, f.Addr().Interface())
							return errors.Wrapf(err, "Could not decode %s into %s", ct, field.Type)
						})
					return false
				}

				name := field.Name // not used by model, but used by the rest
				if tags.name != "" {
					name = tags.name
				}
				unpacker, err := getUnpacker(field.Type, field.Name, name, base, tags, options)
				if err != nil {
					returnError = err
					return false
				}
				switch base {
				case "path":
					varsFillers = append(varsFillers, func(model reflect.Value, vars map[string]string) error {
						f := model.FieldByIndex(field.Index)
						return errors.Wrapf(
							unpacker.single("path", f, vars[name]),
							"path element %s into field %s",
							name, field.Name)
					})
				case "header":
					if unpacker.multi != nil {
						headerFillers = append(headerFillers, func(model reflect.Value, header http.Header) error {
							f := model.FieldByIndex(field.Index)
							values, ok := header[name]
							if !ok {
								return nil
							}
							return errors.Wrapf(
								unpacker.multi("header", f, values),
								"header %s into field %s",
								name, field.Name)
						})
					} else {
						headerFillers = append(headerFillers, func(model reflect.Value, header http.Header) error {
							f := model.FieldByIndex(field.Index)
							values, ok := header[name]
							if !ok || len(values) == 0 {
								return nil
							}
							return errors.Wrapf(
								unpacker.single("header", f, values[0]),
								"header %s into field %s",
								name, field.Name)
						})
					}
				case "query":
					switch {
					case unpacker.deepObject != nil:
						deepObjectFillers[name] = func(model reflect.Value, mapValues map[string][]string) error {
							f := model.FieldByIndex(field.Index)
							return unpacker.deepObject(f, mapValues)
						}
					case unpacker.multi != nil:
						queryFillers[name] = func(model reflect.Value, values []string) error {
							f := model.FieldByIndex(field.Index)
							return errors.Wrapf(
								unpacker.multi("query", f, values),
								"query parameter %s into field %s",
								name, field.Name)
						}
					default:
						queryFillers[name] = func(model reflect.Value, values []string) error {
							if len(values) == 0 {
								return nil
							}
							f := model.FieldByIndex(field.Index)
							return errors.Wrapf(
								unpacker.single("query", f, values[0]),
								"query parameter %s into field %s",
								name, field.Name)
						}
					}
				case "cookie":
					cookieFillers = append(cookieFillers, func(model reflect.Value, r *http.Request) error {
						f := model.FieldByIndex(field.Index)
						cookie, err := r.Cookie(name)
						if err != nil {
							if errors.Is(err, http.ErrNoCookie) {
								return nil
							}
							return errors.Wrapf(err, "cookie parameter %s into field %s", name, field.Name)
						}
						return errors.Wrapf(
							unpacker.single("cookie", f, cookie.Value),
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
				len(cookieFillers) == 0 &&
				len(queryFillers) == 0 &&
				len(bodyFillers) == 0 &&
				len(deepObjectFillers) == 0 {
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
				var deepObjects map[string]map[string][]string
				for key, vals := range r.URL.Query() {
					if qf, ok := queryFillers[key]; ok {
						setError(qf(model, vals))
						continue
					}
					if len(deepObjectFillers) != 0 {
						if m := deepObjectRE.FindStringSubmatch(key); len(m) == 3 {
							if _, ok := deepObjectFillers[m[1]]; ok {
								if deepObjects == nil {
									deepObjects = make(map[string]map[string][]string)
								}
								if deepObjects[m[1]] == nil {
									deepObjects[m[1]] = make(map[string][]string)
								}
								deepObjects[m[1]][m[2]] = vals
								continue
							}
						}
					}
					if options.rejectUnknownQueryParameters {
						setError(errors.Errorf("query parameter '%s' not supported", key))
					}
				}
				for dofKey, values := range deepObjects {
					setError(deepObjectFillers[dofKey](model, values))
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
	base string,
	fieldType reflect.Type,
	tagName string,
	outerTags tags,
	options eigo,
) (unpack, error) {
	type fillTarget struct {
		field reflect.StructField
		unpack
	}
	targets := make(map[string]fillTarget)
	var anyErr error
	reflectutils.WalkStructElements(fieldType, func(field reflect.StructField) bool {
		tag, _ := field.Tag.Lookup(tagName)
		// nolint:govet
		name, tags, err := parseTag(tag, false)
		if err != nil {
			anyErr = errors.Wrap(err, field.Name)
			return false
		}
		switch name {
		case "-":
			return true
		case "":
			name = field.Name
		}
		if _, ok := targets[name]; ok {
			anyErr = errors.Errorf("Only one field can be filled with the same name.  '%s' is duplicated.  One example is %s",
				name, field.Name)
			return false
		}
		if !outerTags.deepObject {
			tags.explode = false
		}
		if tags.deepObject {
			anyErr = errors.Errorf("deepObject=true is not allowed on fields inside a struct.  Used on %s", name)
			return false
		}
		unpacker, err := getUnpacker(field.Type, field.Name, name, base, tags, options)
		if err != nil {
			anyErr = errors.Wrap(err, field.Name)
			return false
		}
		targets[name] = fillTarget{
			field:  field,
			unpack: unpacker,
		}
		return true
	})
	if anyErr != nil {
		return unpack{}, anyErr
	}
	return unpack{
		multi: func(from string, model reflect.Value, values []string) error {
			for i := 0; i < len(values); i += 2 {
				keyString := values[i]
				var valueString string
				if i+1 < len(values) {
					valueString = values[i+1]
				}
				target, ok := targets[keyString]
				if !ok {
					if options.rejectUnknownQueryParameters {
						return errors.Errorf("No struct member to receive key '%s'", keyString)
					}
					continue
				}
				f := model.FieldByIndex(target.field.Index)
				err := target.single(from, f, valueString)
				if err != nil {
					return errors.Wrap(err, target.field.Name)
				}
			}
			return nil
		},
		deepObject: func(model reflect.Value, mapValues map[string][]string) error {
			for keyString, values := range mapValues {
				target, ok := targets[keyString]
				if !ok {
					if options.rejectUnknownQueryParameters {
						return errors.Errorf("No struct member to receive key '%s'", keyString)
					}
					continue
				}
				f := model.FieldByIndex(target.field.Index)
				var err error
				if target.single != nil {
					if len(values) > 0 {
						err = target.single("query", f, values[0])
					}
				} else {
					err = target.multi("query", f, values)
				}
				if err != nil {
					return errors.Wrap(err, target.field.Name)
				}
			}
			return nil
		},
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
		err := keyUnpack(from, keyPointer.Elem(), keyString)
		if err != nil {
			return err
		}
		valuePointer := reflect.New(f.Type().Elem())
		err = valueUnpack(from, valuePointer.Elem(), valueString)
		if err != nil {
			return err
		}
		m.SetMapIndex(reflect.Indirect(keyPointer), reflect.Indirect(valuePointer))
	}
	f.Set(m)
	return nil
}

func sliceUnpack(
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

func arrayUnpack(
	from string, f reflect.Value,
	singleUnpack func(from string, target reflect.Value, value string) error,
	values []string,
) error {
	arrayLen := f.Len()
	if len(values) > arrayLen {
		return errors.New("too many values for fixed length array")
	}
	for i, value := range values {
		err := singleUnpack(from, f.Index(i), value)
		if err != nil {
			return err
		}
	}
	for k := len(values); k < arrayLen; k++ {
		f.Index(k).Set(reflect.Zero(f.Index(0).Type()))
	}
	return nil
}

type unpack struct {
	createMe   bool
	single     func(from string, target reflect.Value, value string) error
	multi      func(from string, target reflect.Value, values []string) error
	deepObject func(target reflect.Value, mapValues map[string][]string) error
}

// getUnpacker is used for unpacking headers, query parameters, and path elements
func getUnpacker(
	fieldType reflect.Type,
	fieldName string,
	name string,
	base string, // "path", "query", etc.
	tags tags,
	options eigo,
) (unpack, error) {
	if tags.content != "" {
		return contentUnpacker(fieldType, fieldName, name, base, tags, options)
	}
	if fieldType.AssignableTo(textUnmarshallerType) {
		return unpack{
			createMe: true,
			single: func(from string, target reflect.Value, value string) error {
				p := reflect.New(fieldType.Elem())
				target.Set(p)
				return errors.Wrapf(
					target.Interface().(encoding.TextUnmarshaler).UnmarshalText([]byte(value)),
					"decode %s %s", from, name)
			},
		}, nil
	}
	if reflect.PtrTo(fieldType).AssignableTo(textUnmarshallerType) {
		return unpack{
			createMe: true,
			single: func(from string, target reflect.Value, value string) error {
				return errors.Wrapf(
					target.Addr().Interface().(encoding.TextUnmarshaler).UnmarshalText([]byte(value)),
					"decode %s %s", from, name)
			},
		}, nil
	}

	switch fieldType.Kind() {
	case reflect.Ptr:
		unpacker, err := getUnpacker(fieldType.Elem(), fieldName, name, base, tags, options)
		if err != nil {
			return unpack{}, err
		}
		switch {
		case unpacker.deepObject != nil:
			return unpack{deepObject: func(target reflect.Value, mapValues map[string][]string) error {
				p := reflect.New(fieldType.Elem())
				target.Set(p)
				return unpacker.deepObject(target.Elem(), mapValues)
			}}, nil
		case unpacker.multi != nil:
			return unpack{multi: func(from string, target reflect.Value, values []string) error {
				p := reflect.New(fieldType.Elem())
				target.Set(p)
				return unpacker.multi(from, target.Elem(), values)
			}}, nil
		default:
			return unpack{single: func(from string, target reflect.Value, value string) error {
				p := reflect.New(fieldType.Elem())
				target.Set(p)
				return unpacker.single(from, target.Elem(), value)
			}}, nil
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uintptr, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64,
		reflect.String,
		reflect.Complex64, reflect.Complex128,
		reflect.Bool:
		f, err := reflectutils.MakeStringSetter(fieldType)
		if err != nil {
			return unpack{}, errors.Wrapf(err, "Cannot decode into %s, %s", fieldName, fieldType)
		}
		return unpack{single: func(from string, target reflect.Value, value string) error {
			return errors.Wrapf(f(target, value), "decode %s %s", from, name)
		}}, nil

	case reflect.Slice, reflect.Array:
		switch base {
		case "cookie", "path":
			if tags.delimiter != "," {
				return unpack{}, errors.New("delimiter setting is only allowed for 'query' parameters")
			}
			if tags.explode {
				return unpack{}, errors.New("explode=true not supported for cookies & path parameters")
			}
		}
		if tags.deepObject {
			return unpack{}, errors.New("deepObject=true not supported for slices")
		}

		singleUnpack, err := getUnpacker(fieldType.Elem(), fieldName, name, base, tags.WithoutExplode(), options)
		if err != nil {
			return unpack{}, err
		}
		unslicer := sliceUnpack
		if fieldType.Kind() == reflect.Array {
			unslicer = arrayUnpack
		}
		switch base {
		case "query", "header":
			if tags.explode {
				return unpack{
					multi: func(from string, target reflect.Value, values []string) error {
						return unslicer(from, target, singleUnpack.single, values)
					},
				}, nil
			}
		}
		return unpack{single: func(from string, target reflect.Value, value string) error {
			values := strings.Split(value, tags.delimiter)
			return unslicer(from, target, singleUnpack.single, values)
		}}, nil

	case reflect.Struct:
		structUnpacker, err := generateStructUnpacker(base, fieldType, options.tag, tags, options)
		if err != nil {
			return unpack{}, err
		}
		if tags.deepObject {
			if base != "query" {
				return unpack{}, errors.Errorf("deepObject=true not supported for %s", base)
			}
			return unpack{deepObject: structUnpacker.deepObject}, nil
		}
		switch base {
		case "query", "header":
			if tags.explode {
				return unpack{
					multi: func(from string, target reflect.Value, values []string) error {
						return structUnpacker.multi(from, target, resplitOnEquals(values))
					},
				}, nil
			}
		}
		return unpack{single: func(from string, target reflect.Value, value string) error {
			values := strings.Split(value, tags.delimiter)
			return structUnpacker.multi(from, target, values)
		}}, nil

	case reflect.Map:
		switch base {
		case "cookie", "path":
			if tags.delimiter != "," {
				return unpack{}, errors.New("delimiter setting is only allowed for 'query' parameters")
			}
		}
		keyUnpack, err := getUnpacker(fieldType.Key(), fieldName, name, base, tags.WithoutExplode().WithoutDeepObject(), options)
		if err != nil {
			return unpack{}, err
		}
		etags := tags
		if tags.deepObject {
			etags = etags.WithoutDeepObject()
		} else {
			etags = etags.WithoutExplode()
		}
		elementUnpack, err := getUnpacker(fieldType.Elem(), fieldName, name, base, etags, options)
		if err != nil {
			return unpack{}, err
		}
		if tags.deepObject {
			if base != "query" {
				return unpack{}, errors.Errorf("deepObject=true not supported for %s", base)
			}
			return unpack{deepObject: func(target reflect.Value, mapValues map[string][]string) error {
				m := reflect.MakeMap(fieldType)
				for keyString, values := range mapValues {
					keyPointer := reflect.New(fieldType.Key())
					err := keyUnpack.single("query", keyPointer.Elem(), keyString)
					if err != nil {
						return err
					}
					valuePointer := reflect.New(fieldType.Elem())
					if elementUnpack.multi != nil {
						err = elementUnpack.multi("query", valuePointer.Elem(), values)
					} else {
						var valueString string
						if len(values) > 0 {
							valueString = values[0]
						}
						err = elementUnpack.single("query", valuePointer.Elem(), valueString)
					}
					if err != nil {
						return err
					}
					m.SetMapIndex(reflect.Indirect(keyPointer), reflect.Indirect(valuePointer))
				}
				target.Set(m)
				return nil
			}}, nil
		}
		switch base {
		case "query", "header":
			if tags.explode {
				return unpack{
					multi: func(from string, target reflect.Value, values []string) error {
						return mapUnpack(from, target, keyUnpack.single, elementUnpack.single, resplitOnEquals(values))
					},
				}, nil
			}
		}
		return unpack{single: func(from string, target reflect.Value, value string) error {
			values := strings.Split(value, tags.delimiter)
			return mapUnpack(from, target, keyUnpack.single, elementUnpack.single, values)
		}}, nil

	case reflect.Chan, reflect.Interface, reflect.UnsafePointer, reflect.Func, reflect.Invalid:
		fallthrough
	default:
		return unpack{}, errors.Errorf(
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
	options eigo,
) (unpack, error) {
	decoder, ok := options.decoders[tags.content]
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
			return unpack{}, errors.Errorf("No decoder provided for content type '%s'", tags.content)
		}
	}
	kind := fieldType.Kind()
	if tags.explode &&
		(base == "query" || base == "header") &&
		(kind == reflect.Map || kind == reflect.Slice) {
		valueUnpack, err := getUnpacker(fieldType.Elem(), fieldName, name, base, tags.WithoutExplode(), options)
		if err != nil {
			return unpack{}, err
		}
		if kind == reflect.Slice {
			return unpack{multi: func(from string, target reflect.Value, values []string) error {
				a := reflect.MakeSlice(target.Type(), len(values), len(values))
				for i, valueString := range values {
					// nolint:govet
					err := valueUnpack.single(from, a.Index(i), valueString)
					if err != nil {
						return err
					}
				}
				target.Set(a)
				return nil
			}}, nil
		}
		keyUnpack, err := getUnpacker(fieldType.Key(), fieldName, name, base, tags.WithoutExplode().WithoutContent().WithoutDeepObject(), options)
		if err != nil {
			return unpack{}, err
		}
		return unpack{multi: func(from string, target reflect.Value, values []string) error {
			m := reflect.MakeMap(target.Type())
			for _, pair := range values {
				kv := strings.SplitN(pair, "=", 2)
				keyString := kv[0]
				var valueString string
				if len(kv) == 2 {
					valueString = kv[1]
				}
				keyPointer := reflect.New(fieldType.Key())
				err := keyUnpack.single(from, keyPointer, keyString)
				if err != nil {
					return err
				}
				valuePointer := reflect.New(fieldType.Elem())
				err = valueUnpack.single(from, valuePointer, valueString)
				if err != nil {
					return err
				}
				m.SetMapIndex(reflect.Indirect(keyPointer), reflect.Indirect(valuePointer))
			}
			target.Set(m)
			return nil
		}}, nil
	}

	return unpack{single: func(from string, target reflect.Value, value string) error {
		i := target.Addr().Interface()
		err := decoder([]byte(value), i)
		return errors.Wrap(err, fieldName)
	}}, nil
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
	deepObject    bool
}

func (tags tags) WithoutExplode() tags    { tags.explode = false; return tags }
func (tags tags) WithoutContent() tags    { tags.content = ""; return tags }
func (tags tags) WithoutDeepObject() tags { tags.deepObject = false; return tags }

func parseTag(s string, validate bool) (string, tags, error) {
	a := strings.Split(s, ",")
	// nolint:govet
	var tags tags
	if len(a) == 0 {
		return "", tags, errors.New("must specify the source of the data ('path', 'query', etc)")
	}
	tags.delimiter = ","
	if validate {
		switch a[0] {
		case "path":
		case "query":
			tags.explode = true
		case "header":
			tags.explode = true
		case "cookie":
		case "model":
		default:
			return "", tags, errors.Errorf("'%s' is not a valid source of the data use ('model', 'path', 'query', etc)", a[0])
		}
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
		case "deepObject":
			tags.deepObject = true
		default:
			return "", tags, errors.Errorf("tag %s is not supported", k)
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
