package nject

import (
	"reflect"
)

func (fm *provider) DownFlows() ([]reflect.Type, []reflect.Type) {
	switch fm.class {
	case unsetClassType:
		// continue
	default:
		return fm.flows[inputParams].Types(), fm.flows[outputParams].Types()
	}
	switch r := fm.fn.(type) {
	case Reflective:
		return reflectiveEffectiveOutputs(r)
	case generatedFromInjectionChain:
		return nil, nil
	}
	v := reflect.ValueOf(fm.fn)
	if !v.IsValid() {
		return nil, nil
	}
	t := v.Type()
	if t.Kind() == reflect.Func {
		switch fm.group {
		case finalGroup:
			return typesIn(t), nil
		default:
			return effectiveOutputs(t)
		}
	}
	if fm.group == invokeGroup && t.Kind() == reflect.Ptr && t.Elem().Kind() == reflect.Func {
		return nil, typesIn(t.Elem())
	}
	return nil, []reflect.Type{t}
}

func reflectiveEffectiveOutputs(r Reflective) ([]reflect.Type, []reflect.Type) {
	fn := wrappedReflective{r}
	if w, ok := r.(ReflectiveWrapper); ok {
		in := typesIn(fn)
		// discard the first type because it's the inner()
		return in[1:], typesIn(wrappedReflective{w.Inner()})
	}
	return effectiveOutputs(fn)
}

// The inputs to inner() are additional types that are provided
// downstream.
func effectiveOutputs(fn reflectType) ([]reflect.Type, []reflect.Type) {
	inputs := typesIn(fn)
	outputs := typesOut(fn)
	if len(inputs) == 0 || inputs[0].Kind() != reflect.Func {
		for i := len(outputs) - 1; i >= 0; i-- {
			out := outputs[i]
			if out == terminalErrorType {
				outputs = append(outputs[:i], outputs[i+1:]...)
			}
		}
		return inputs, outputs
	}
	i0 := inputs[0]
	inputs = inputs[1:]
	return inputs, typesIn(i0)
}

func (c Collection) netFlows(f func(fm *provider) ([]reflect.Type, []reflect.Type)) ([]reflect.Type, []reflect.Type) {
	available := make(interfaceMap)
	seenIn := make(map[reflect.Type]struct{})
	uniqueIn := make([]reflect.Type, 0, len(c.contents)*4)
	seenOut := make(map[reflect.Type]struct{})
	uniqueOut := make([]reflect.Type, 0, len(c.contents)*4)
	for i, fm := range c.contents {
		inputs, outputs := f(fm)
		inputsByType := make(map[reflect.Type]struct{})
		for _, input := range inputs {
			t, _, err := available.bestMatch(getTypeCode(input), "input")
			if err == nil {
				input = t.Type()
			}
			inputsByType[input] = struct{}{}
			if _, ok := seenOut[input]; ok {
				continue
			}
			if _, ok := seenIn[input]; ok {
				continue
			}
			seenIn[input] = struct{}{}
			uniqueIn = append(uniqueIn, input)
		}
		for _, output := range outputs {
			available.Add(getTypeCode(output), i, fm)
			if _, ok := inputsByType[output]; ok {
				continue
			}
			if _, ok := seenIn[output]; ok {
				continue
			}
			if _, ok := seenOut[output]; ok {
				continue
			}
			seenOut[output] = struct{}{}
			uniqueOut = append(uniqueOut, output)
		}
	}
	return uniqueIn, uniqueOut
}

// DownFlows provides the net unresolved flows down the injection chain.
// If a type is used both as input and as output for the same provider,
// then that type counts as an input only.
func (c Collection) DownFlows() ([]reflect.Type, []reflect.Type) {
	return c.netFlows(func(fm *provider) ([]reflect.Type, []reflect.Type) {
		return fm.DownFlows()
	})
}

func (fm *provider) UpFlows() ([]reflect.Type, []reflect.Type) {
	switch fm.class {
	case unsetClassType:
		// continue
	default:
		return fm.flows[receivedParams].Types(), fm.flows[returnParams].Types()
	}
	switch r := fm.fn.(type) {
	case Reflective:
		return reflectiveEffectiveReturns(r)
	case generatedFromInjectionChain:
		return nil, nil
	}
	v := reflect.ValueOf(fm.fn)
	if !v.IsValid() {
		return nil, nil
	}
	t := v.Type()
	if t.Kind() == reflect.Func {
		switch fm.group {
		case finalGroup:
			return nil, typesOut(t)
		default:
			return effectiveReturns(t)
		}
	}
	if fm.group == invokeGroup && t.Kind() == reflect.Ptr && t.Elem().Kind() == reflect.Func {
		return typesOut(t.Elem()), nil
	}
	return nil, nil
}

func reflectiveEffectiveReturns(r Reflective) ([]reflect.Type, []reflect.Type) {
	fn := wrappedReflective{r}
	if w, ok := r.(ReflectiveWrapper); ok {
		return typesOut(wrappedReflective{w.Inner()}), typesOut(fn)
	}
	return effectiveReturns(fn)
}

// Only wrapper functions consume return values and only
// wrapper functions provide return values
func effectiveReturns(fn reflectType) ([]reflect.Type, []reflect.Type) {
	inputs := typesIn(fn)
	if len(inputs) == 0 || inputs[0].Kind() != reflect.Func {
		for _, out := range typesOut(fn) {
			if out == terminalErrorType {
				return nil, []reflect.Type{errorType}
			}
		}
		return nil, nil
	}
	i0 := inputs[0]
	return typesOut(i0), typesOut(fn)
}

// UpFlows provides the net unresolved flows up the injection chain.
// If a type is used both as value it consumes as a return value and also
// as a value that it in turn returns, then the up flow for that provider will
// be counted only by what it consumes.
//
// Providers that return TerminalError are a special case and count as
// producing error.
func (c Collection) UpFlows() ([]reflect.Type, []reflect.Type) {
	return c.netFlows(func(fm *provider) ([]reflect.Type, []reflect.Type) {
		return fm.UpFlows()
	})
}

// stripUnused redacts unusedType from lists
func stripUnused(list []reflect.Type) []reflect.Type {
	for i, t := range list {
		if t == unusedType {
			n := make([]reflect.Type, i, len(list)-1)
			copy(n, list[:i])
			for _, t := range list[i+1:] {
				if t != unusedType {
					n = append(n, t)
				}
			}
			return n
		}
	}
	return list
}

// stripUnusedCode redacts unusedTypeCode from lists
func stripUnusedCodes(list []typeCode) []typeCode {
	for i, t := range list {
		if t == unusedTypeCode {
			n := make([]typeCode, i, len(list)-1)
			copy(n, list[:i])
			for _, t := range list[i+1:] {
				if t != unusedTypeCode {
					n = append(n, t)
				}
			}
			return n
		}
	}
	return list
}
