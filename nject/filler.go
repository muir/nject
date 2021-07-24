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

// makeFiller creates an object that implements Reflective.  It is used by
// MakeStructBuilder and MakeStructFiller
func makeFiller(model interface{}, tag string, create bool) (Reflective, bool, error) {
	t := reflect.TypeOf(model)
	if debugFiller {
		fmt.Println("filler type", t.String())
	}
	ot := t
	f := filler{}
	if t.Kind() == reflect.Ptr {
		f.pointer = true
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil, false, fmt.Errorf("MakeStructFiller must be called with a struct or pointer to struct, not %T", model)
	}
	f.typ = t
	f.inputs = make([]inputDisposition, 0, t.NumField()+1)
	var addIgnore bool
	if !create {
		f.inputs = append(f.inputs, inputDisposition{
			typ: ot,
		})
		f.copy = true
		if !f.pointer {
			return nil, false, fmt.Errorf("Cannot fill an existing struct that is not a pointer.  Called with %T", model)
		}
	} else if t.NumField() > 0 && t.Field(0).Type.Kind() == reflect.Func {
		// uh, oh, we don't want to take a function
		f.inputs = append(f.inputs, inputDisposition{
			typ: ignoreType,
		})
		addIgnore = true
	}
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
			var whole bool
			if tag != "" {
				ps := field.Tag.Get(tag)
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
							return fmt.Errorf("Invalid struct tag '%s' on %s for building struct filler", tv, field.Name)
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
		return nil, false, err
	}
	return &f, addIgnore, nil
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
