package nject

import (
	"fmt"
	"reflect"
	"strings"
	"unicode"
	"unicode/utf8"
)

const debugFiller = false

// filler tracks how to fill a struct, it is an example implementation
// of Reflective
type filler struct {
	pointer bool
	create  bool
	copy    bool
	typ     reflect.Type
	inputs  []inputDisposition
}

// inputDisposition is how one input to Call() gets stored
// into a struct
type inputDisposition struct {
	mapping []int
	typ     reflect.Type
}

var _ Reflective = &filler{}

// canCall is either a reflect.Value, a filler, or a Reflective
type canCall interface {
	Call([]reflect.Value) []reflect.Value
}

func getCanCall(f interface{}) canCall {
	if r, ok := f.(Reflective); ok {
		return r
	}
	return reflect.ValueOf(f)
}

type canRealType interface {
	Type() reflect.Type
}
type canFakeType interface {
	In(int) reflect.Type
}

// getInZero is used to get the first input type for a function
// that is in a canCall.  This depends upon knowing that canCall
// is either a reflect.Type or a Reflective
func getInZero(cc canCall) reflect.Type {
	if t, ok := cc.(canRealType); ok {
		return t.Type().In(0)
	}
	if t, ok := cc.(canFakeType); ok {
		return t.In(0)
	}
	return nil
}

type fillerOptions struct {
	tag            string
	postMethodName string
	postAction     map[string]interface{}
	create         bool
}

var reservedTags = map[string]struct{}{
	"fill":   {},
	"-":      {},
	"whole":  {},
	"blob":   {},
	"fields": {},
}

// MakeStructBuilder generates a Provider that wants to receive as
// arguments all of the fields of the struct and returns the struct
// as what it provides.
//
// The input model must be a struct: if not MakeStructFiller
// will panic.  Model may be a pointer to a struct or a struct.
// Unexported fields are always ignored.
// Passing something other than a struct or pointer to a struct to
// MakeStructBuilder results is an error. Unknown tag values is an error.
//
// Struct tags can be used to control the
// behavior: the argument controls the name of the struct tag used.
// A struct tag of "-" or "ignore" indicates that the field should not
// be filled.  A tag of "fill" is accepted but doesn't do anything as it's
// the default.
//
// Embedded structs can either be filled as a whole or they can be
// filled field-by-field.  Tag with "whole" or "blob" to fill the embedded
// struct all at once.  Tag with "fields" to fill the fields of the
// embedded struct individually.  The default is "fields".
func MakeStructBuilder(model interface{}, optArgs ...FillerFuncArg) (Provider, error) {
	options := fillerOptions{
		tag:        "nject",
		create:     true,
		postAction: make(map[string]interface{}),
	}
	for _, f := range optArgs {
		f(&options)
	}
	for tag := range options.postAction {
		if _, ok := reservedTags[tag]; ok {
			return nil, fmt.Errorf("Tag value '%s' is reserved and cannot be used by PostAction", tag)
		}
	}
	t := reflect.TypeOf(model)
	if debugFiller {
		fmt.Println("filler type", t.String())
	}
	originalType := t
	f := filler{}
	if t.Kind() == reflect.Ptr {
		f.pointer = true
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("MakeStructFiller must be called with a struct or pointer to struct, not %T", model)
	}
	f.typ = t
	f.inputs = make([]inputDisposition, 0, t.NumField()+1)
	var addIgnore bool
	if !options.create {
		f.inputs = append(f.inputs, inputDisposition{
			typ: originalType,
		})
		f.copy = true
		if !f.pointer {
			return nil, fmt.Errorf("Cannot fill an existing struct that is not a pointer.  Called with %T", model)
		}
	} else if t.NumField() > 0 && t.Field(0).Type.Kind() == reflect.Func {
		// uh, oh, we don't want to take a function
		f.inputs = append(f.inputs, inputDisposition{
			typ: ignoreType,
		})
		addIgnore = true
	}
	var mapStruct func(t reflect.Type, path []int) error
	var additionalReflectives []interface{}
	mapStruct = func(t reflect.Type, path []int) error {
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			np := copyIntSlice(path)
			np = append(np, field.Index...)
			if debugFiller {
				fmt.Printf("Field %d of %s: %s %v\n", i, t, field.Name, np)
			}
			r, _ := utf8.DecodeRuneInString(field.Name)
			if !unicode.IsUpper(r) {
				continue
			}
			var skip bool
			var whole bool
			if options.tag != "" {
				ps := field.Tag.Get(options.tag)
				if ps != "" {
					for _, tv := range strings.Split(ps, ",") {
						switch tv {
						case "fill":
							skip = false
						case "-", "ignore":
							skip = true
						case "whole", "blob":
							if field.Type.Kind() == reflect.Struct {
								whole = true
							} else {
								return fmt.Errorf("Cannot use tag %s on type %s (%s) for building struct filler",
									tv, field.Name, field.Type)
							}
						case "fields":
							if field.Type.Kind() == reflect.Struct {
								whole = false
							} else {
								return fmt.Errorf("Cannot use tag %s on type %s (%s) for building struct filler",
									tv, field.Name, field.Type)
							}
						default:
							if fun, ok := options.postAction[tv]; ok {
								ap, err := addFieldFiller(np, field, originalType, fun, tv)
								if err != nil {
									return err
								}
								additionalReflectives = append(additionalReflectives, ap)
							} else {
								return fmt.Errorf("Invalid struct tag '%s' on %s for building struct filler", tv, field.Name)
							}
						}
					}
				}
			}
			if skip {
				continue
			}
			if field.Type.Kind() == reflect.Struct && !whole {
				err := mapStruct(field.Type, np)
				if err != nil {
					return err
				}
				continue
			}
			if debugFiller {
				fmt.Printf(" map input %d (%s) to %s %v\n", len(f.inputs), field.Type.String(), field.Name, np)
			}
			f.inputs = append(f.inputs, inputDisposition{
				typ:     field.Type,
				mapping: np,
			})
		}
		return nil
	}
	err := mapStruct(t, []int{})
	if err != nil {
		return nil, err
	}
	p := Provide(fmt.Sprintf("builder for %T", model), &f)
	var chain []interface{}
	if addIgnore {
		chain = append(chain, ignore{})
	}
	chain = append(chain, p)
	chain = append(chain, additionalReflectives...)
	if len(chain) == 1 {
		return p, nil
	}
	return Cluster(fmt.Sprintf("builder seq for %T", model), chain...), nil
}

func (f *filler) In(i int) reflect.Type {
	return f.inputs[i].typ
}
func (f *filler) NumIn() int {
	return len(f.inputs)
}
func (f *filler) Out(i int) reflect.Type {
	if f.pointer {
		return reflect.PtrTo(f.typ)
	}
	return f.typ
}
func (f *filler) NumOut() int {
	return 1
}

func (f *filler) Call(inputs []reflect.Value) []reflect.Value {
	var v reflect.Value
	var r reflect.Value
	if f.copy {
		if debugFiller {
			fmt.Println("filling ", f.typ.String())
		}
		r = inputs[0]
		if f.pointer {
			if r.IsNil() {
				fmt.Println(" IS NIL")
			}
			v = r.Elem()
		} else {
			fmt.Println(" not pointer")
			v = r
		}
	} else {
		if debugFiller {
			fmt.Println("creating ", f.typ.String())
		}
		r = reflect.New(f.typ)
		v = r.Elem()
		if !f.pointer {
			r = v
		}
	}
	for i, input := range inputs {
		disposition := f.inputs[i]
		if disposition.mapping == nil {
			continue
		}
		fv := v.FieldByIndex(disposition.mapping)
		if debugFiller {
			fmt.Printf(" input %d: about to set %v %s (%T -> %T)\n",
				i, disposition.mapping, v.Type().FieldByIndex(disposition.mapping).Name,
				input.Type().String(), fv.Type().String())
		}
		fv.Set(input)
	}
	return []reflect.Value{r}
}

// reflectiveWrapper allows Refelective to kinda pretend to be a reflect.Type
type reflectiveWrapper struct {
	Reflective
}

// reflecType is a subset of reflect.Type good enough for use in characterize
type reflectType interface {
	Kind() reflect.Kind
	NumOut() int
	NumIn() int
	In(i int) reflect.Type
	Elem() reflect.Type
	Out(i int) reflect.Type
	String() string
}

var _ reflectType = reflectiveWrapper{}

func (w reflectiveWrapper) Kind() reflect.Kind { return reflect.Func }
func (w reflectiveWrapper) Elem() reflect.Type { panic("call not expected") }

func (w reflectiveWrapper) String() string {
	in := make([]string, w.NumIn())
	for i := 0; i < w.NumIn(); i++ {
		in[i] = w.In(i).String()
	}
	out := make([]string, 1, w.NumOut())
	for i := 0; i < w.NumOut(); i++ {
		out[i] = w.Out(i).String()
	}
	switch len(out) {
	case 0:
		return "Reflective(" + strings.Join(in, ", ") + ")"
	case 1:
		return "Reflective(" + strings.Join(in, ", ") + ") " + out[0]
	default:
		return "Reflective(" + strings.Join(in, ", ") + ") (" + strings.Join(out, ", ") + ")"
	}
}

func copyIntSlice(in []int) []int {
	c := make([]int, len(in), len(in)+1)
	copy(c, in)
	return c
}

type thinReflective struct {
	inputs  []reflect.Type
	outputs []reflect.Type
	fun     func([]reflect.Value) []reflect.Value
}

var _ Reflective = thinReflective{}

func (r thinReflective) In(i int) reflect.Type                   { return r.inputs[i] }
func (r thinReflective) NumIn() int                              { return len(r.inputs) }
func (r thinReflective) Out(i int) reflect.Type                  { return r.outputs[i] }
func (r thinReflective) NumOut() int                             { return len(r.outputs) }
func (r thinReflective) Call(in []reflect.Value) []reflect.Value { return r.fun(in) }

// addFIeldFiller creates a Reflective that wraps a closure that
// unpacks the recently filled struct and extracts the relevant field
// from that struct and substitutes that into the inputs to
// the provided function to be called on a field of the struct
// post-filling.
func addFieldFiller(path []int, field reflect.StructField, outerStruct reflect.Type, fun interface{}, tagValue string) (Provider, error) {
	funcAsValue := reflect.ValueOf(fun)
	if !funcAsValue.IsValid() {
		return nil, fmt.Errorf("PostAction(%s, func) for filling %s was called with something other than a function",
			tagValue, outerStruct)
	}
	t := funcAsValue.Type()
	if t.Kind() != reflect.Func {
		return nil, fmt.Errorf("PostAction(%s, func) for filling %s was called with a %s instead of with a function",
			tagValue, outerStruct, t)
	}
	inputs := typesIn(t)
	const bad = 1000000 // a number larger than the number of Methods that an interface might have
	const convert = 750000
	const assign = 500000
	var score int = bad // negative is best
	var inputIndex int
	var addressOf bool
	var needConvert bool
	for i, in := range inputs {
		check := func(t reflect.Type, aOf bool) {
			var s int
			var c bool
			switch {
			case in == t:
				s = -1
			case t.AssignableTo(in):
				if in.Kind() != reflect.Interface {
					s = assign
				} else {
					s = in.NumMethod()
				}
			case t.ConvertibleTo(in):
				s = convert
				c = true
			default:
				return
			}
			if s >= score {
				return
			}
			score = s
			inputIndex = i
			addressOf = aOf
			needConvert = c
		}
		check(field.Type, false)
		check(reflect.PtrTo(field.Type), true)
	}
	if score == bad {
		return nil, fmt.Errorf("PostAction(%s, func) for filling %s, no match found between field type %s and function inputs",
			tagValue, outerStruct, field.Type)
	}
	targetType := inputs[inputIndex]
	inputs[inputIndex] = outerStruct
	structIsPtr := outerStruct.Kind() == reflect.Ptr
	if needConvert && addressOf {
		return nil, fmt.Errorf("PostAction(%s, func) for filling %s, matched %s to input %s (converting) but that cannot be combined with conversion to a pointer",
			tagValue, outerStruct, field.Type, targetType)
	}

	return Provide(fmt.Sprintf("fill-%s-of-%s", field.Name, outerStruct),
		thinReflective{
			inputs:  inputs,
			outputs: typesOut(t),
			fun: func(in []reflect.Value) []reflect.Value {
				strct := in[inputIndex]
				if structIsPtr {
					strct = strct.Elem()
				}
				v := strct.FieldByIndex(path)
				if addressOf {
					v = v.Addr()
				}
				if needConvert {
					v = v.Convert(targetType)
				}
				in[inputIndex] = v
				return funcAsValue.Call(in)
			},
		}), nil
}
