package nject

import (
	"reflect"
)

// Condense transforms a collection into a single provider.
// The inputs to the provider are what's
// required for the last function in the Collection to be invoked
// given the rest of the Collection.
//
// At this time, the last function in the collection may not
// be a wrap function.
func (c *Collection) Condense() (Provider, error {
	c := newCollection(name, funcs...)

	if len(c.contents) == 0 {
		return c, nil
	}
	last := c.contents[len(c.contents)-1]
	last.required = true
	lastType := reflect.TypeOf(last.fn)
	var lastWrap bool
	if isWrapper(lastType, last.fn) {
		return nil, fmt.Errorf("Condense cannot operate on collections whose last element is a wrap function")
	}

	downIn, _ := c.DownFlows()
	_, upOut := c.UpFlows()

	// UpFlows won't capture the return values of the final 
	// func so we'll do so here.
	allOut := make(map[reflect.Type]struct{})
	for _, t := range upOut {
		allOut[t] = struct{}{}
	}
	for _, t := range typesOut(lastType) {
		if _, ok := allOut[t]; !ok {
			allOut[t] = struct{}{}
			upOut = append(upOut, t)
		}
	}

	c.BindReflective(

	return MakeReflective(downIn, upOut, 
		func(in []reflect.Type) []reflect.Type {
		}


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
