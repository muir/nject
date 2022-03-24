package nject

import (
	"fmt"
	"reflect"
	"sync"
)

// When !real, do not actually bind.  !real is used for generating debug traces.
func doBind(sc *Collection, originalInvokeF *provider, originalInitF *provider, real bool) error {
	// Split up the collection into LITERAL, STATIC, RUN, and FINAL groups. Add
	// init and invoke as faked providers.  Flatten into one ordered list.
	var invokeIndex int
	var invokeF *provider
	var initF *provider
	var debuggingProvider **provider
	funcs := make([]*provider, 0, len(sc.contents)+3)
	{
		var err error
		invokeF, err = characterizeInitInvoke(originalInvokeF, charContext{inputsAreStatic: false})
		if err != nil {
			return err
		}
		if invokeF.flows == nil {
			return fmt.Errorf("internal error #4: no flows for invoke")
		}
		nonStaticTypes := make(map[typeCode]bool)
		for _, tc := range invokeF.flows[outputParams] {
			nonStaticTypes[tc] = true
		}

		beforeInvoke, afterInvoke, err := sc.characterizeAndFlatten(nonStaticTypes)
		if err != nil {
			return err
		}

		err = checkForMissingOverridesError(afterInvoke)
		if err != nil {
			return err
		}

		// Add debugging provider
		{
			d := newProvider(func() *Debugging { return nil }, -1, "Debugging")
			d.cacheable = true
			d.mustCache = true
			d, err = characterizeFunc(d, charContext{inputsAreStatic: true})
			if err != nil {
				return fmt.Errorf("internal error #29: problem with debugging injectors: %w", err)
			}
			d.isSynthetic = true
			debuggingProvider = &d
			funcs = append(funcs, d)
		}

		// Add init
		if originalInitF != nil {
			initF, err = characterizeInitInvoke(originalInitF, charContext{inputsAreStatic: true})
			if err != nil {
				return err
			}
			if initF.flows == nil {
				return fmt.Errorf("internal error #5: no flows for initF")
			}
			funcs = append(funcs, initF)
		}

		funcs = append(funcs, beforeInvoke...)
		invokeIndex = len(funcs)
		funcs = append(funcs, invokeF)
		funcs = append(funcs, afterInvoke...)

		for i, fm := range funcs {
			fm.chainPosition = i
			if fm.required {
				fm.include = true
			}
		}
	}

	// Figure out which providers must be included in the final chain.  To do this,
	// first we figure out where each provider will get its inputs from when going
	// down the chain and where its inputs can be consumed when going up the chain.
	// Each of these linkages will be recorded as a dependency.  Any dependency that
	// cannot be met will result in that provider being marked as impossible to
	// include.
	//
	// After all the dependencies are mapped, then we mark which providers will be
	// included in the final chain.
	//
	// The parameter list for the init function is complicated: both the inputs
	// and outputs are associated with downVmap, but they happen at different times:
	// some of the bookkeeping related to init happens in sequence with its position
	// in the function list, and some of it happens just before handling the invoke
	// function.
	//
	//
	// When that is finished, we can compute the upVmap and the downVmap.

	// Compute dependencies: set fm.downRmap, fm.upRmap, fm.cannotInclude,
	// fm.whyIncluded, fm.include
	err := computeDependenciesAndInclusion(funcs, initF)
	if err != nil {
		return err
	}

	// Build the lists of parameters that are included in the value collections.
	// These are maps from types to position in the value collection.
	//
	// Also: calculate bypass zero for static chain.  If there is a fallible injector
	// in the static chain, then part of the static chain my not run.  Fallible
	// injectors need to know know which types need to be zeroed if the remaining
	// static injectors are skipped.
	//
	// Also: calculate the skipped-inner() zero for the run chain.  If a wrapper
	// does not call the remainder of the chain, then the values returned by the remainder
	// of the chain must be zero'ed.
	downVmap := make(map[typeCode]int)
	upVmap := make(map[typeCode]int)
	vCount := 0 // combined count of up and down parameters
	for _, fm := range funcs {
		if !fm.include {
			continue
		}
		for _, flow := range fm.flows {
			for _, tc := range flow {
				upVmap[tc] = -1
				downVmap[tc] = -1
			}
		}
	}
	// calculate for the static set
	for i := invokeIndex - 1; i >= 0; i-- {
		fm := funcs[i]
		fm.mustZeroIfRemainderSkipped = vmapMapped(downVmap)
		addToVmap(fm, outputParams, downVmap, fm.downRmap, &vCount)
	}
	if initF != nil {
		for _, tc := range initF.flows[bypassParams] {
			if rm, found := initF.downRmap[tc]; found {
				tc = rm
			}
			if downVmap[tc] == -1 {
				return fmt.Errorf("Type required by init func, %s, not provided by any static group injectors", tc)
			}
		}
	}
	// calculate for the run set
	for i := len(funcs) - 1; i >= invokeIndex; i-- {
		fm := funcs[i]
		fm.vmapCount = vCount
		addToVmap(fm, inputParams, downVmap, fm.downRmap, &vCount)
		addToVmap(fm, returnParams, upVmap, fm.upRmap, &vCount)
		fm.mustZeroIfInnerNotCalled = vmapMapped(upVmap)
	}

	// Fill in debugging (if used)
	if (*debuggingProvider).include {
		(*debuggingProvider).fn = func() *Debugging {
			included := make([]string, 0, len(funcs)+3)
			for _, fm := range funcs {
				if fm.include {
					included = append(included, fmt.Sprintf("%s %s", fm.group, fm))
				}
			}

			namesIncluded := make([]string, 0, len(funcs)+3)
			for _, fm := range funcs {
				if fm.include {
					if fm.index >= 0 {
						namesIncluded = append(namesIncluded, fmt.Sprintf("%s(%d)", fm.origin, fm.index))
					} else {
						namesIncluded = append(namesIncluded, fm.origin)
					}
				}
			}

			includeExclude := make([]string, 0, len(funcs)+3)
			for _, fm := range funcs {
				if fm.include {
					includeExclude = append(includeExclude, fmt.Sprintf("INCLUDED: %s %s BECAUSE %s", fm.group, fm, fm.whyIncluded))
				} else {
					includeExclude = append(includeExclude, fmt.Sprintf("EXCLUDED: %s %s BECAUSE %s", fm.group, fm, fm.cannotInclude))
				}
			}

			var trace string
			if debugEnabled() {
				trace = "debugging already in progress"
			} else {
				trace = captureDoBindDebugging(sc, originalInvokeF, originalInitF)
			}

			reproduce := generateReproduce(funcs, invokeF, initF)

			return &Debugging{
				Included:       included,
				NamesIncluded:  namesIncluded,
				IncludeExclude: includeExclude,
				Trace:          trace,
				Reproduce:      reproduce,
			}
		}
	}
	if debugEnabled() {
		for _, fm := range funcs {
			dumpF("funclist", fm)
		}
	}

	// Generate wrappers and split the handlers into groups (static, middleware, final)
	collections := make(map[groupType][]*provider)
	for _, fm := range funcs {
		if !fm.include {
			continue
		}
		err := generateWrappers(fm, downVmap, upVmap)
		if err != nil {
			return err
		}
		collections[fm.group] = append(collections[fm.group], fm)
	}
	if len(collections[finalGroup]) != 1 {
		return fmt.Errorf("internal error #1: no final func provided")
	}

	// Over the course of the following loop, f will be redefined
	// over and over so that at the end of the loop it will be a
	// function that executes the entire RUN chain.  We start with
	// an f that calls the final provider and work backwards.
	f := collections[finalGroup][0].wrapEndpoint
	for i := len(collections[runGroup]) - 1; i >= 0; i-- {
		n := collections[runGroup][i]

		// nolint:exhaustive
		switch n.class {
		case wrapperFunc:
			inner := f
			w := n.wrapWrapper
			f = func(v valueCollection) {
				w(v, inner)
			}
		case injectorFunc, fallibleInjectorFunc:
			// For injectors that aren't wrappers, we iterate rather than nest.
			j := i - 1
		Injectors:
			for j >= 0 {
				// nolint:exhaustive
				switch collections[runGroup][j].class {
				default:
					break Injectors
				case injectorFunc, fallibleInjectorFunc: // okay
				}
				j--
			}
			j++
			next := f
			injectors := make([]func(valueCollection) bool, 0, i-j+1)
			for k := j; k <= i; k++ {
				injectors = append(injectors, collections[runGroup][k].wrapFallibleInjector)
			}
			f = func(v valueCollection) {
				for _, injector := range injectors {
					errored := injector(v)
					if errored {
						return
					}
				}
				next(v)
			}
			i = j
		default:
			return fmt.Errorf("internal error #2: should not be here: %s", n.class)
		}
	}

	// Initialize the value collection.   When invoke is called the baseValues
	// collection will be copied.
	baseValues := make(valueCollection, vCount)
	for _, lit := range collections[literalGroup] {
		i := downVmap[lit.flows[outputParams][0]]
		if i >= 0 {
			baseValues[i] = reflect.ValueOf(lit.fn)
		}
	}

	// Generate static chain function
	runStaticChain := func() error {
		debugf("STATIC CHAIN LENGTH: %d", len(collections[staticGroup]))
		for _, inj := range collections[staticGroup] {
			debugf("STATIC CHAIN CALLING %s", inj)

			err := inj.wrapStaticInjector(baseValues)
			if err != nil {
				debugf("STATIC CHAIN RETURNING EARLY DUE TO ERROR %s", err)
				return err
			}
		}
		return nil
	}
	for _, inj := range collections[staticGroup] {
		if inj.wrapStaticInjector == nil {
			return inj.errorf("internal error #3: missing static injector wrapping")
		}
	}

	// Generate and bind init func.
	initFunc := func() {}
	var initOnce sync.Once
	if initF != nil {
		outMap, err := generateOutputMapper(initF, 0, outputParams, downVmap, "init inputs")
		if err != nil {
			return err
		}

		inMap, err := generateInputMapper(initF, 0, bypassParams, initF.bypassRmap, downVmap, "init results")
		if err != nil {
			return err
		}

		debugln("SET INIT FUNC")
		if real {
			reflect.ValueOf(initF.fn).Elem().Set(
				reflect.MakeFunc(reflect.ValueOf(initF.fn).Type().Elem(),
					func(inputs []reflect.Value) []reflect.Value {
						debugln("INSIDE INIT")
						// if initDone panic, return error, or ignore?
						initOnce.Do(func() {
							outMap(baseValues, inputs)
							debugln("RUN STATIC CHAIN")
							_ = runStaticChain()
						})
						dumpValueArray(baseValues, "base values before init return", downVmap)
						out := inMap(baseValues)
						debugln("DONE INIT")
						dumpValueArray(out, "init return", nil)
						dumpF("init", initF)

						return out
					}))
		}
		debugln("SET INIT FUNC - DONE")

	} else {
		initFunc = func() {
			initOnce.Do(func() {
				_ = runStaticChain()
			})
		}
	}

	// Generate and bind invoke func
	{
		outMap, err := generateOutputMapper(invokeF, 0, outputParams, downVmap, "invoke inputs")
		if err != nil {
			return err
		}

		inMap, err := generateInputMapper(invokeF, 0, returnedParams, invokeF.upRmap, upVmap, "invoke results")
		if err != nil {
			return err
		}

		debugln("SET INVOKE FUNC")
		if real {
			reflect.ValueOf(invokeF.fn).Elem().Set(
				reflect.MakeFunc(reflect.ValueOf(invokeF.fn).Type().Elem(),
					func(inputs []reflect.Value) []reflect.Value {
						initFunc()
						values := baseValues.Copy()
						dumpValueArray(values, "invoke - before input copy", downVmap)
						outMap(values, inputs)
						dumpValueArray(values, "invoke - after input copy", downVmap)
						f(values)
						return inMap(values)
					}))
		}
		debugln("SET INVOKE FUNC - DONE")
	}

	return nil
}

func vmapMapped(vMap map[typeCode]int) []typeCode {
	used := make([]typeCode, 0, len(vMap))
	for tc, i := range vMap {
		if i >= 0 {
			used = append(used, tc)
		}
	}
	return used
}

func addToVmap(fm *provider, param flowType, vMap map[typeCode]int, rMap map[typeCode]typeCode, counter *int) {
	for _, tc := range fm.flows[param] {
		if rm, found := rMap[tc]; found {
			tc = rm
		}
		if vMap[tc] == -1 {
			vMap[tc] = *counter
			*counter++
		}
	}
}
