package nject

import (
	"fmt"
	"reflect"
)

// Curry generates a Requird Provider that prefills arguments to a function to create a
// new function that needs fewer args.
//
// Only arguments with a unique (to the function) type can be curried.
//
// The original function and the curried function must have the same outputs.
//
// The first curried input may not be a function.
//
// EXPERIMENTAL: This is currently considered experimental and could be removed or
// moved to another package. If you're using this, open a pull request to remove
// this comment.
func Curry(originalFunction interface{}, pointerToCurriedFunction interface{}) (Provider, error) {
	o := reflect.ValueOf(originalFunction)
	if !o.IsValid() {
		return nil, fmt.Errorf("original function is not a valid value")
	}
	if o.Type().Kind() != reflect.Func {
		return nil, fmt.Errorf("first argument to Curry must be a function")
	}
	n := reflect.ValueOf(pointerToCurriedFunction)
	if !n.IsValid() {
		return nil, fmt.Errorf("curried function is not a valid value")
	}
	if n.Type().Kind() != reflect.Ptr {
		return nil, fmt.Errorf("curried function must be a pointer (to a function)")
	}
	if n.Type().Elem().Kind() != reflect.Func {
		return nil, fmt.Errorf("curried function must be a pointer to a function")
	}
	if n.IsNil() {
		return nil, fmt.Errorf("pointer to curried function cannot be nil")
	}
	if o.Type().NumOut() != n.Type().Elem().NumOut() {
		return nil, fmt.Errorf("current function doesn't have the same number of outputs, %d, as curried function, %d",
			o.Type().NumOut(), n.Type().Elem().NumOut())
	}
	outputs := make([]reflect.Type, o.Type().NumOut())
	for i := 0; i < len(outputs); i++ {
		if o.Type().Out(i) != n.Type().Elem().Out(i) {
			return nil, fmt.Errorf("current function return value #%d has a different type, %s, than the curried functions return value, %s",
				i+1, o.Type().Out(i), n.Type().Elem().Out(i))
		}
		outputs[i] = o.Type().Out(i)
	}

	// Figure out the set of input types for the curried function
	ntypes := make(map[reflect.Type][]int)
	for i := 0; i < n.Type().Elem().NumIn(); i++ {
		t := n.Type().Elem().In(i)
		ntypes[t] = append(ntypes[t], i)
	}

	// Now, for each input in the original function, figure out where it
	// is coming from.
	originalNumIn := o.Type().NumIn()
	used := make(map[reflect.Type]int)
	curryCount := originalNumIn - n.Type().Elem().NumIn()
	if curryCount < 1 {
		return nil, fmt.Errorf("curried function must take fewer arguments than original function")
	}
	curried := make([]reflect.Type, 0, curryCount)    // injected inputs
	alreadyCurried := make(map[reflect.Type]struct{}) // to prevent double-dipping
	curryMap := make([]int, 0, curryCount)            // maps postion from injected inputs to to original
	passMap := make([]int, n.Type().Elem().NumIn())   // maps position from curried to original
	for i := 0; i < o.Type().NumIn(); i++ {
		t := o.Type().In(i)
		if plist, ok := ntypes[t]; ok {
			if used[t] < len(plist) {
				passMap[plist[used[t]]] = i
				used[t]++
			} else {
				return nil, fmt.Errorf("original function takes more arguments of type %s than the curried function", t)
			}
		} else {
			if _, ok := alreadyCurried[t]; ok {
				return nil, fmt.Errorf("cannot curry the same type (%s) more than once", t)
			}
			alreadyCurried[t] = struct{}{}
			curryMap = append(curryMap, i)
			curried = append(curried, t)
		}
	}
	for t, plist := range ntypes {
		if used[t] < len(plist) {
			return nil, fmt.Errorf("not all of the %s inputs to the curried function were used by the original", t)
		}
	}
	if len(curried) > 0 && curried[0].Kind() == reflect.Func {
		return nil, fmt.Errorf("the first curried input, %s, may not be a function", curried[0])
	}

	var fillInjected func(oi []reflect.Value)

	curryFunc := func(inputs []reflect.Value) []reflect.Value {
		oi := make([]reflect.Value, originalNumIn)
		for i, in := range inputs {
			oi[passMap[i]] = in
		}
		fillInjected(oi)
		return o.Call(oi)
	}

	return Required(MakeReflective(curried, nil, func(inputs []reflect.Value) []reflect.Value {
		fillInjected = func(oi []reflect.Value) {
			for i, in := range inputs {
				oi[curryMap[i]] = in
			}
		}
		n.Elem().Set(reflect.MakeFunc(n.Type().Elem(), curryFunc))
		return nil
	})), nil
}

// MustSaveTo calls FillVars and panics if FillVars returns an error
//
// EXPERIMENTAL: This is currently considered experimental and could be removed or
// moved to another package. If you're using this, open a pull request to remove
// this comment.
func MustSaveTo(varPointers ...interface{}) Provider {
	p, err := SaveTo(varPointers...)
	if err != nil {
		panic(err)
	}
	return p
}

// SaveTo generates a required provider.  The input parameters to FillVars
// must be pointers.  The generated provider takes as inputs the types needed
// to assign through the pointers.
//
// If you want to fill a struct, use MakeStructBuilder() instead.
//
// The first argument to FillVars may not be a pointer to a function.
func SaveTo(varPointers ...interface{}) (Provider, error) {
	inputs := make([]reflect.Type, len(varPointers))
	pointers := make([]reflect.Value, len(varPointers))
	for i, vp := range varPointers {
		v := reflect.ValueOf(vp)
		if !v.IsValid() {
			return nil, fmt.Errorf("argument %d of FillVars, is not a valid pointer", i)
		}
		if v.Type().Kind() != reflect.Ptr {
			return nil, fmt.Errorf("argument %d of FillVars, a %s, is not a pointer and thus invalid", i, v.Type())
		}
		if v.IsNil() {
			return nil, fmt.Errorf("argument %d of FillVars, a %s, is nil and thus invalid", i, v.Type())
		}
		if !v.Elem().CanSet() {
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
			pointers[i].Elem().Set(v)
		}
		return nil
	})), nil
}

// MustCurry calls Curry and panics if Curry returns error
func MustCurry(originalFunction interface{}, pointerToCurriedFunction interface{}) Provider {
	p, err := Curry(originalFunction, pointerToCurriedFunction)
	if err != nil {
		panic(err)
	}
	return p
}
