package nject

import (
	"fmt"
	"reflect"
)

type valueCollection []reflect.Value

// This maps a parameter list (either inputs or outputs) into the up or down
// valueCollection.
type parameterMap struct {
	vcIndex []int          // maps param index to value collection index
	types   []reflect.Type // maps param index to param type
	len     int            // length of the above since it's used
}

func (v valueCollection) Copy() valueCollection {
	dup := make(valueCollection, len(v))
	copy(dup, v)
	return dup
}

// generageFlowMap generates a mapping between the parameter list indexes and positions within
// a valueCollection.
func generateParameterMap(
	fm *provider,
	param flowType, // which parameter list to map to/from
	start int, // offset into the parameter list to start mapping
	rmap map[typeCode]typeCode, // type overrides for parameters vs valueCollection
	vmap map[typeCode]int, // map of types in valueCollection to their positions
	purpose string, // debug string
) (parameterMap, error) {
	flow := fm.flows[param]
	m := make([]int, len(flow))
	types := make([]reflect.Type, len(flow))
	for i, p := range flow {
		if i < start && p != noTypeCode {
			return parameterMap{}, fm.errorf("internal error #13: expecting notype for skipped %s, %d %s", param, i, purpose)
		}
		if p == noTypeCode && i >= start {
			return parameterMap{}, fm.errorf("internal error #14: expecting skip for notype %s, %d %s", param, i, purpose)
		}
		if p != noTypeCode {
			var useP typeCode
			if rmap == nil {
				useP = p
			} else {
				var found bool
				useP, found = rmap[p]
				if !found {
					return parameterMap{}, fm.errorf("internal error #7: rmap incomplete %s %d %s missing %s", param, i, purpose, p)
				}
			}
			vci, found := vmap[useP]
			if !found {
				return parameterMap{}, fm.errorf("internal error #8: vmap incomplete %s %d %s missing %s was %s", param, i, purpose, useP, p)
			}
			m[i] = vci
			types[i] = useP.Type()
		}
	}
	return parameterMap{
		vcIndex: m,
		types:   types,
		len:     len(flow),
	}, nil
}

// generateInputMapper returns a function that copies values from valueCollection to an array of reflect.Value
func generateInputMapper(fm *provider, start int, param flowType, rmap map[typeCode]typeCode, vmap map[typeCode]int, purpose string) (func(valueCollection) []reflect.Value, error) {
	pMap, err := generateParameterMap(fm, param, start, rmap, vmap, purpose+" valueCollection->[]")
	if err != nil {
		return nil, err
	}

	return func(v valueCollection) []reflect.Value {
		if debugEnabled() {
			debugf("%s: %s [%s] numIn:%d, m:%v", fm, param, formatFlow(fm.flows[param]), pMap.len, pMap.vcIndex)
		}
		dumpValueArray(v, "", vmap)
		in := make([]reflect.Value, pMap.len)
		for i := start; i < pMap.len; i++ {
			if pMap.vcIndex[i] != -1 {
				in[i] = v[pMap.vcIndex[i]]
				if !in[i].IsValid() {
					in[i] = reflect.Zero(pMap.types[i])
				}
			}
		}
		return in
	}, nil
}

// generateOutputMapper returns a function that copies values from an array of reflect.Value to a valueCollection
// nolint:unparam
func generateOutputMapper(fm *provider, start int, param flowType, vmap map[typeCode]int, purpose string) (func(valueCollection, []reflect.Value), error) {
	pMap, err := generateParameterMap(fm, param, start, nil, vmap, purpose+" []->valueCollection")
	if err != nil {
		return nil, err
	}
	return func(v valueCollection, out []reflect.Value) {
		for i := start; i < pMap.len; i++ {
			if pMap.vcIndex[i] != -1 {
				v[pMap.vcIndex[i]] = out[i]
				if !v[pMap.vcIndex[i]].IsValid() {
					v[pMap.vcIndex[i]] = reflect.Zero(pMap.types[i])
				}
			}
		}
	}, nil
}

func makeZero(fm *provider, vMap map[typeCode]int, mustZero []typeCode, context string) (func(v valueCollection), error) {
	zeroMap := make(map[int]reflect.Type)
	newMap := make(map[int]reflect.Type)
	done := make(map[typeCode]bool)
	for _, p := range mustZero {
		if done[p] {
			continue
		}
		done[p] = true
		i, found := vMap[p]
		if !found {
			return nil, fm.errorf("internal error #9: no type mapping for %s that must be zeroed", p)
		}
		if reflect.Zero(p.Type()).CanInterface() {
			zeroMap[i] = p.Type()
		} else if reflect.New(p.Type()).CanAddr() && reflect.New(p.Type()).Elem().CanInterface() {
			newMap[i] = p.Type()
		} else if !fm.callsInner {
			return nil, fm.errorf("cannot create useful zero value for %s (%s)", p, context)
		}
	}
	return func(v valueCollection) {
		for i, typ := range zeroMap {
			v[i] = reflect.Zero(typ)
		}
		for i, typ := range newMap {
			v[i] = reflect.New(typ).Elem()
		}
	}, nil
}

func terminalErrorIndex(fn reflectType) (int, error) {
	for i, t := range typesOut(fn) {
		if t == terminalErrorType {
			return i, nil
		}
	}
	return -1, fmt.Errorf("internal error #10: Could not find TerminalError in output")
}

func generateWrappers(
	fm *provider,
	downVmap map[typeCode]int, // value collection map for variables passed down
	upVmap map[typeCode]int, // value collection map for return values coming up
) error {
	fv := getCanCall(fm.fn)

	switch fm.class {
	case finalFunc:
		inMap, err := generateInputMapper(fm, 0, inputParams, fm.downRmap, downVmap, "in(final)")
		if err != nil {
			return err
		}
		upMap, err := generateOutputMapper(fm, 0, returnParams, upVmap, "up(final)")
		if err != nil {
			return err
		}
		fm.wrapEndpoint = func(v valueCollection) {
			in := inMap(v)
			upMap(v, fv.Call(in))
		}

	case wrapperFunc:
		inMap, err := generateInputMapper(fm, 1, inputParams, fm.downRmap, downVmap, "in(w)") // parmeters to the middleware handler
		if err != nil {
			return err
		}
		outMap, err := generateOutputMapper(fm, 0, outputParams, downVmap, "out(w)") // parameters to inner()
		if err != nil {
			return err
		}
		upMap, err := generateOutputMapper(fm, 0, returnParams, upVmap, "up(w)") // return values from handler
		if err != nil {
			return err
		}
		retMap, err := generateInputMapper(fm, 0, receviedParams, fm.upRmap, upVmap, "ret(w)") // return values from inner()
		if err != nil {
			return err
		}
		zero, err := makeZero(fm, upVmap, fm.mustZeroIfInnerNotCalled, "upMap(w)")
		if err != nil {
			return err
		}
		in0Type := getInZero(fv)
		in0Reflective, isInnerReflective := in0Type.(Reflective)
		fm.wrapWrapper = func(v valueCollection, next func(valueCollection)) {
			vCopy := v.Copy()
			callCount := 0

			rTypes := make([]reflect.Type, len(fm.flows[receviedParams]))
			for i, tc := range fm.flows[receviedParams] {
				rTypes[i] = tc.Type()
			}

			// for thread safty, this is not built outside WrapWrapper
			inner := func(i []reflect.Value) []reflect.Value {
				if callCount > 0 {
					copy(v, vCopy)
				}
				callCount++
				outMap(v, i)
				next(v)
				r := retMap(v)
				for i, retV := range r {
					if rTypes[i].Kind() == reflect.Interface {
						r[i] = retV.Convert(rTypes[i])
					}
				}
				return r
			}
			in := inMap(v)
			if in0Reflective {
				XXXX
			} else {
				in[0] = reflect.MakeFunc(in0Type, inner)
				defer func() {
					if r := recover(); r != nil {
						zero(v)
						panic(r)
					}
				}()
				out := fv.Call(in)
			}
			if callCount == 0 {
				zero(v)
			}
			upMap(v, out)
		}

	case fallibleInjectorFunc:
		inMap, err := generateInputMapper(fm, 0, inputParams, fm.downRmap, downVmap, "in(fallible)")
		if err != nil {
			return err
		}
		outMap, err := generateOutputMapper(fm, 0, outputParams, downVmap, "out(fallible)")
		if err != nil {
			return err
		}
		zero, err := makeZero(fm, upVmap, fm.mustZeroIfInnerNotCalled, "up(fallible)")
		if err != nil {
			return err
		}
		errorIndex, err := terminalErrorIndex(getReflectType(fm.fn))
		if err != nil {
			return err
		}
		upVerrorIndex := upVmap[getTypeCode(errorType)]
		var memoized cacherFunc
		if fm.memoized {
			memoized = generateCache(fm.id, fv, len(fm.flows[inputParams]), fm.mapKeyCheck)
		}
		fm.wrapFallibleInjector = func(v valueCollection) bool {
			in := inMap(v)
			var out []reflect.Value
			if fm.memoized {
				out = memoized(in)
			} else {
				out = fv.Call(in)
			}
			if out[errorIndex].Interface() != nil {
				zero(v)
				v[upVerrorIndex] = out[errorIndex].Convert(errorType)
				if debugEnabled() {
					debugln("ABOUT TO RETURN ERROR")
					dumpValueArray(v, "error return", upVmap)
				}
				return true
			}
			outMap(v, append(out[:errorIndex], out[errorIndex+1:]...))
			debugln("ABOUT TO RETURN NIL")
			return false
		}

	case injectorFunc:
		inMap, err := generateInputMapper(fm, 0, inputParams, fm.downRmap, downVmap, "in")
		if err != nil {
			return err
		}
		outMap, err := generateOutputMapper(fm, 0, outputParams, downVmap, "out")
		if err != nil {
			return err
		}
		if fm.memoized {
			memoized := generateCache(fm.id, fv, len(fm.flows[inputParams]), fm.mapKeyCheck)
			fm.wrapFallibleInjector = func(v valueCollection) bool {
				in := inMap(v)
				outMap(v, memoized(in))
				return false
			}
		} else {
			fm.wrapFallibleInjector = func(v valueCollection) bool {
				in := inMap(v)
				outMap(v, fv.Call(in))
				return false
			}
		}

	case staticInjectorFunc:
		inMap, err := generateInputMapper(fm, 0, inputParams, fm.downRmap, downVmap, "in")
		if err != nil {
			return err
		}
		outMap, err := generateOutputMapper(fm, 0, outputParams, downVmap, "out")
		if err != nil {
			return err
		}
		lookup := generateLookup(fm, fv, len(fm.flows[inputParams]))
		fm.wrapStaticInjector = func(v valueCollection) error {
			in := inMap(v)
			var out []reflect.Value
			if lookup != nil {
				out = lookup(in)
			} else {
				out = fv.Call(in)
			}
			outMap(v, out)
			return nil
		}

	case fallibleStaticInjectorFunc:
		inMap, err := generateInputMapper(fm, 0, inputParams, fm.downRmap, downVmap, "in")
		if err != nil {
			return err
		}
		outMap, err := generateOutputMapper(fm, 0, outputParams, downVmap, "out")
		if err != nil {
			return err
		}
		errorIndex, err := terminalErrorIndex(getReflectType(fm.fn))
		if err != nil {
			return err
		}
		zero, err := makeZero(fm, downVmap, fm.mustZeroIfRemainderSkipped, "need to fill in values set in skippped functions")
		if err != nil {
			return err
		}
		lookup := generateLookup(fm, fv, len(fm.flows[inputParams]))
		fm.wrapStaticInjector = func(v valueCollection) error {
			debugf("RUNNING %s", fm)
			in := inMap(v)
			var out []reflect.Value
			if lookup != nil {
				out = lookup(in)
			} else {
				out = fv.Call(in)
			}
			err := out[errorIndex].Interface() // this is a TerminalError
			out[errorIndex] = out[errorIndex].Convert(errorType)
			outMap(v, out)
			if err != nil {
				if debugEnabled() {
					debugf("Zeroing for %s", fm)
					dumpValueArray(v, "BEFORE", downVmap)
				}
				zero(v)
				if debugEnabled() {
					dumpValueArray(v, "AFTER", downVmap)
					debugf("RETURNING %v", err)
				}
				return err.(error)
			}
			if debugEnabled() {
				debugf("NOT zeroing for %s", fm)
				debugf("RETURNING nil")
			}
			return nil
		}

	case invokeFunc, initFunc, literalValue:
		// handled elsewhere
		return nil
	case unsetClassType:
		fallthrough
	default:
		return fmt.Errorf("internal error #11: unexpected class")
	}
	return nil
}
