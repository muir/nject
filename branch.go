package nject

import (
	"reflect"
)

// Branch creates a Collection and transforms it into a single
// Required provider. The inputs to the collapsed function are what's
// required for the last function in the Collection to be invoked
// given the rest of the Collection.
//
// The last function can be a wrap function.  Any inputs to the
// inner function will be supplied to the main (or surrounding)
// injection chain.
// Any outputs of the inner function will be supplied by the
// main (or surrounding) injectection chain.
//
// The squashed Collection is a wrap function if there are any
// wrap functions inside the Collection.
//
// Branch providers can include Branch providers inside themselves.
func Branch(name string, funcs ...interface{}) Provider {
	c := newCollection(name, funcs...)
	return GenerateFromInjectionChain(name, func(chainBefore Collection, chainAfter Collection) (selfReplacement Provider, err error) {
		if len(c.contents) == 0 {
			return c, nil
		}
		last := c.contents[len(c.contents)-1]
		last.required = true
		lastType := reflect.TypeOf(last.fn)
		var lastWrap bool
		if lastType.Kind() == reflect.Func && lastType.NumIn() > 0 && lastType.In(0).Kind() == reflect.Func {
			lastWrap = true
			standinForMainChain := newProvider(MakeReflective(
				typesIn(lastType.In(0)),
				typesOut(lastType.In(0)),
				func(in []reflect.Value) []reflect.Value {
					panic("this should not actually be used")
				}))
			c.contents = append(c.contents, standinForMainChain)
		}

		nonStaticTypes := make(map[typeCode]bool)
		for _, input := range inputs {
			nonStaticTypes[getTypeCode(input)] = true
		}
		p1, p2, err := c.characterizeAndFlatten(nonStaticTypes)
		if err != nil {
			return nil, err
		}

		funcs := make([]*provider, 0, len(c.contents)+5)

		d := makeDebuggingProvider()
		funcs = append(funcs, d)
		funcs = append(funcs, p1...)
		funcs = append(funcs, p2...)

		funcs, err = computeDependenciesAndInclusion(funcs, nil)
		if err != nil {
			return nil, err
		}

		includedFuncs := make([]interface{}, 0, len(funcs))
		wraps := lastWrap
		for _, fm := range funcs {
			// XXX does d get copied?
			if fm.include && fm != d {
				includedFuncs = append(includedFuncs, fm.fn)
				if fm.class == wrapperFunc {
					wraps = true
				}
			}
		}

		nc := Sequence(name, includedFuncs...)
		inputs, outputs := nc.netFlows()

		if d.include {
			inputs = append(inputs, debuggingType.PtrTo())
		}

		invoke := MakeReflective(inputs, nil, nil)
		err := c.Bind(invoke, nil)

	})
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