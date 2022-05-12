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

type canInner interface {
	Inner() Reflective
}

func getReflectType(i interface{}) reflectType {
	if r, ok := i.(Reflective); ok {
		w := reflectiveWrapper{r}
		return w
	}
	return reflect.TypeOf(i)
}

// getInZero is used to get the first input type for a function
// that is in a canCall.  This depends upon knowing that canCall
// is either a reflect.Type, a Reflective, or a ReflectiveWrapper
func getInZero(cc canCall) (reflect.Type, Reflective) {
	if t, ok := cc.(canRealType); ok {
		return t.Type().In(0), nil
	}
	if t, ok := cc.(canInner); ok {
		return nil, t.Inner()
	}
	if t, ok := cc.(canFakeType); ok {
		return t.In(0), nil
	}
	return nil, nil
}

type fillerOptions struct {
	tag              string
	postMethodName   []string
	postActionByTag  map[string]postActionOption
	postActionByName map[string]postActionOption
	postActionByType []postActionOption
	create           bool
}

var reservedTags = map[string]struct{}{
	"whole":  {},
	"blob":   {},
	"fields": {},
	"field":  {},
	"-":      {},
	"skip":   {},
	"nofill": {},
	"fill":   {},
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
//
// The following struct tags are pre-defined.  User-created struct tags
// (created with PostActionByTag) may not uses these names:
//
// "whole" & "blob": indicate that an embedded struct should be filled as
// a blob rather thatn field-by-field.
//
// "field" & "fields": indicates that an embedded struct should be filled
// field-by-field.  This is the default and the tag exists for clarity.
//
// "-" & "skip": the field should not be filled and it should should ignore a
// PostActionByType and PostActionByName matches.
// PostActionByTag would still apply.
//
// "nofill": the field should not be filled, but all PostActions still
// apply.  "nofill" overrides other behviors including defaults set with
// post-actions.
//
// "fill": normally if there is a PostActionByTag match, then the field
// will not be filled from the provider chain.  "fill" overrides that
// behavior.  "fill" overrides other behaviors including defaults set with
// post-actions.
//
// If you just want to provide a value variable, use FillVars() instead.
func MakeStructBuilder(model interface{}, optArgs ...FillerFuncArg) (Provider, error) {
	// Options handling
	options := fillerOptions{
		tag:              "nject",
		create:           true,
		postActionByTag:  make(map[string]postActionOption),
		postActionByName: make(map[string]postActionOption),
	}
	for _, f := range optArgs {
		f(&options)
	}
	for tag := range options.postActionByTag {
		if _, ok := reservedTags[tag]; ok {
			return nil, fmt.Errorf("Tag value '%s' is reserved and cannot be used by PostAction", tag)
		}
	}
	byType := make(map[typeCode][]postActionOption)
	for _, fun := range options.postActionByType {
		v := reflect.ValueOf(fun.function)
		if !v.IsValid() || v.Type().Kind() != reflect.Func || v.Type().NumIn() == 0 {
			return nil, fmt.Errorf("PostActionByType for %T called with an invalid value: %T", model, fun)
		}
		in0 := getTypeCode(v.Type().In(0))
		byType[in0] = append(byType[in0], fun)
	}

	// Type verification
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

	// model creation
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

	// Field handling.  A closure so that it can be invoked recursively
	// since that's how you have to traverse nested structures.
	var additionalReflectives []interface{}
	var mapStruct func(t reflect.Type, path []int) error
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
			var noSkip bool
			var whole bool
			var hardSkip bool
			handleFieldFiller := func(fun postActionOption, description string) error {
				if hardSkip {
					return nil
				}
				ap, filledWithPtr, err := addFieldFiller(np, field, originalType, fun, description)
				if err != nil {
					return err
				}
				if fun.fillSet {
					if !noSkip {
						skip = !fun.fill
					}
				} else if filledWithPtr {
					if !noSkip {
						skip = true
					}
				}
				additionalReflectives = append(additionalReflectives, ap)
				return nil
			}

			if options.tag != "" {
				ps := field.Tag.Get(options.tag)
				if ps != "" {
					for _, tv := range strings.Split(ps, ",") {
						switch tv {
						case "nofill":
							skip = true
						case "fill":
							skip = false
							noSkip = true
						case "-", "skip":
							skip = true
							hardSkip = true
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
							// PostActionByTag
							if fun, ok := options.postActionByTag[tv]; ok {
								err := handleFieldFiller(fun, fmt.Sprintf(
									"PostActionByTag(%s, func) for %s", tv, originalType))
								if err != nil {
									return err
								}
							} else {
								return fmt.Errorf("Invalid struct tag '%s' on %s for building struct filler", tv, field.Name)
							}
						}
					}
				}
			}

			if fun, ok := options.postActionByName[field.Name]; ok {
				err := handleFieldFiller(fun, fmt.Sprintf(
					"PostActionByName(%s, func) for %s", field.Name, originalType))
				if err != nil {
					return err
				}
			}

			// PostActionByType
			fieldTypes := []reflect.Type{field.Type}
			if f.pointer {
				fieldTypes = []reflect.Type{reflect.PtrTo(field.Type), field.Type}
			}
			for _, typ := range fieldTypes {
				for _, fun := range byType[getTypeCode(typ)] {
					err := handleFieldFiller(fun,
						fmt.Sprintf("PostActionByType(%s) for %s", fun.function, originalType))
					if err != nil {
						return err
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

	// Build the Provider/Cluster
	p := Provide(fmt.Sprintf("builder for %T", model), &f)
	var chain []interface{}
	if addIgnore {
		chain = append(chain, ignore{})
	}
	chain = append(chain, p)
	chain = append(chain, additionalReflectives...)
	for _, name := range options.postMethodName {
		m, err := generatePostMethod(originalType, name)
		if err != nil {
			return nil, err
		}
		chain = append(chain, m)
	}
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
	ReflectiveArgs
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
	out := make([]string, w.NumOut())
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

// MakeReflective is a simple wrapper to create a Reflective
func MakeReflective(
	inputs []reflect.Type,
	outputs []reflect.Type,
	function func([]reflect.Value) []reflect.Value,
) Reflective {
	return thinReflective{
		inputs:  inputs,
		outputs: outputs,
		fun:     function,
	}
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

// addFieldFiller creates a Reflective that wraps a closure that
// unpacks the recently filled struct and extracts the relevant field
// from that struct and substitutes that into the inputs to
// the provided function to be called on a field of the struct
// post-filling.
func addFieldFiller(
	path []int,
	field reflect.StructField,
	outerStruct reflect.Type,
	option postActionOption,
	context string,
) (Provider, bool, error) {
	funcAsValue := reflect.ValueOf(option.function)
	if !funcAsValue.IsValid() {
		return nil, false, fmt.Errorf("%s was called with something other than a function", context)
	}
	t := funcAsValue.Type()
	if t.Kind() != reflect.Func {
		return nil, false, fmt.Errorf("%s was called with a %s instead of with a function", context, t)
	}
	inputs := typesIn(t)
	const bad = 1000000 // a number larger than the number of Methods that an interface might have
	const convert = 750000
	const assign = 500000
	var score int = bad // negative is best
	var inputIndex int
	var addressOf bool
	var needConvert bool
	var countEmptyInterfaces int
	for i, in := range inputs {
		i, in := i, in
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
		if option.matchToInterface {
			if in == emptyInterfaceType {
				countEmptyInterfaces++
				inputIndex = i
				addressOf = true
			}
			continue
		}
		check(field.Type, false)
		check(reflect.PtrTo(field.Type), true)
	}
	if option.matchToInterface && countEmptyInterfaces != 1 {
		return nil, false, fmt.Errorf("%s need exactly one interface{} parameters in function", context)
	}
	if score == bad {
		return nil, false, fmt.Errorf("%s no match found between field type %s and function inputs",
			context, field.Type)
	}
	targetType := inputs[inputIndex]
	inputs[inputIndex] = outerStruct
	structIsPtr := outerStruct.Kind() == reflect.Ptr
	if needConvert && addressOf {
		return nil, false, fmt.Errorf(" %s, matched %s to input %s (converting) but that cannot be combined with conversion to a pointer",
			context, field.Type, targetType)
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
		}), addressOf, nil
}

func generatePostMethod(modelType reflect.Type, methodName string) (Provider, error) {
	method, ok := modelType.MethodByName(methodName)
	if !ok {
		return nil, fmt.Errorf("WIthPostMethod(%s) on %s: no such method exists", methodName, modelType)
	}
	desc := fmt.Sprintf("%s.%s()", modelType, methodName)
	mt := method.Func.Type()
	switch {
	case mt.In(0) == modelType:
		return Provide(desc, thinReflective{
			inputs:  typesIn(mt),
			outputs: typesOut(mt),
			fun: func(in []reflect.Value) []reflect.Value {
				return method.Func.Call(in)
			},
		}), nil
	case modelType.Kind() == reflect.Ptr && mt.In(0) == modelType.Elem():
		inputs := typesIn(mt)
		inputs[0] = modelType
		return Provide(desc, thinReflective{
			inputs:  inputs,
			outputs: typesOut(mt),
			fun: func(in []reflect.Value) []reflect.Value {
				in[0] = in[0].Elem()
				return method.Func.Call(in)
			},
		}), nil
	default:
		return nil, fmt.Errorf("internal error #36: no match between model %s and method input %s",
			modelType, method.Type.In(0))
	}
}
