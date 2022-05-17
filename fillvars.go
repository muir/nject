package nject

import (
	"fmt"
	"reflect"
)

// FillVars generates a required provider.  The input parameters to FillVars
// must be pointers.  The generated provider takes as inputs the types needed
// to assign through the pointers.
//
// If you want to fill a struct, use MakeStructBuilder() instead.
//
// The first argument to FillVars may not be a pointer to a function.
func FillVars(varPointers ...interface{}) (Provider, error) {
	inputs := make([]reflect.Type, len(varPointers))
	pointers := make([]reflect.Value, len(varPointers))
	for i, vp := range varPointers {
		v := reflect.ValueOf(vp)
		if !v.IsValid() {
			return nil, fmt.Errorf("argument %d of FillVars, is not a valid pointer", i)
		}
		if v.IsNil() {
			return nil, fmt.Errorf("argument %d of FillVars, a %s, is nil and thus invalid", i, v.Type())
		}
		if v.Type().Kind() != reflect.Ptr {
			return nil, fmt.Errorf("argument %d of FillVars, a %s, is not a pointer and thus invalid", i, v.Type())
		}
		if !v.CanSet() {
			return nil, fmt.Errorf("argument %d of FillVars, a %s, is not settable and thus invalid", i, v.Type())
		}
		if i == 0 && v.Type().Elem().Kind() == reflect.Func {
			return nil, fmt.Errorf("first argument of FillVars, a %s, may not be a pointer to a function", v.Type())
		}
		inputs[i] = v.Type().Elem()
		pointers[i] = v
	}
	return Required(MakeReflective(inputs, nil, func(in []reflect.Value) []reflect.Value {
		for i, v := range in {
			pointers[i].Set(v)
		}
		return nil
	})), nil
}

// MustFillVars calls FillVars and panics if FillVars returns an error
func MustFillVars(varPointers ...interface{}) Provider {
	p, err := FillVars(varPointers...)
	if err != nil {
		panic(err)
	}
	return p
}
