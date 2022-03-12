package nject

import (
	"fmt"
	"reflect"
	"strings"
)

type charContext struct {
	isLast          bool
	inputsAreStatic bool
}

type flowMapType map[flowType][]typeCode

type characterization struct {
	name   string
	tests  predicates
	mutate func(testArgs)
}

type typeRegistry []characterization

type testArgs struct {
	cc    charContext
	fm    *provider
	t     reflectType
	isNil bool
}

type predicateType struct {
	message string
	test    func(a testArgs) bool
}

type predicates []predicateType

func hasAnonymousFuncs(params []reflect.Type, ignoreFirst bool) bool {
	for i, in := range params {
		if in.Kind() == reflect.Func && in.Name() == "" && !(i == 0 && ignoreFirst) {
			return true
		}
	}
	return false
}

func typesIn(t reflectType) []reflect.Type {
	if t.Kind() != reflect.Func {
		return nil
	}
	in := make([]reflect.Type, t.NumIn())
	for i := 0; i < t.NumIn(); i++ {
		in[i] = t.In(i)
	}
	return in
}

func typesOut(t reflectType) []reflect.Type {
	if t.Kind() != reflect.Func {
		return nil
	}
	out := make([]reflect.Type, t.NumOut())
	for i := 0; i < t.NumOut(); i++ {
		out[i] = t.Out(i)
	}
	return out
}

func remapTerminalError(in []reflect.Type) []reflect.Type {
	out := make([]reflect.Type, len(in))
	for i, t := range in {
		if t == terminalErrorType {
			t = errorType
		}
		out[i] = t
	}
	return out
}

func redactTerminalError(in []reflect.Type) []reflect.Type {
	var out []reflect.Type
	for _, t := range in {
		if t == terminalErrorType {
			continue
		}
		out = append(out, t)
	}
	return out
}

func toTypeCodes(in []reflect.Type) []typeCode {
	out := make([]typeCode, len(in))
	for i, t := range in {
		out[i] = getTypeCode(t)
	}
	return out
}

func mappable(inputs ...reflect.Type) bool {
	ok := true
	for _, in := range inputs {
		// nolint:exhaustive
		switch in.Kind() {
		case reflect.Map, reflect.Slice, reflect.Func:
			ok = false
		case reflect.Array:
			ok = mappable(in.Elem())
		case reflect.Struct:
			fa := make([]reflect.Type, in.NumField())
			for i := 0; i < len(fa); i++ {
				fa[i] = in.Field(i).Type
			}
			ok = mappable(fa...)
		}
		if !ok {
			break
		}
	}
	return ok
}

// predicate tests a provider.  The message is used when the provider
// fails that test so the message should be the opposite of what the
// provider does.
func predicate(message string, test func(a testArgs) bool) predicateType {
	return predicateType{
		message: message,
		test:    test,
	}
}

var (
	notNil               = predicate("is nil", func(a testArgs) bool { return !a.isNil })
	notFunc              = predicate("is a function", func(a testArgs) bool { return a.t.Kind() != reflect.Func })
	isFunc               = predicate("is not a function", func(a testArgs) bool { return a.t.Kind() == reflect.Func })
	isLast               = predicate("is not the final item in the provider chain", func(a testArgs) bool { return a.cc.isLast })
	notLast              = predicate("must not be last", func(a testArgs) bool { return !a.cc.isLast })
	unstaticOkay         = predicate("is marked MustCache", func(a testArgs) bool { return !a.fm.mustCache })
	inStatic             = predicate("is after invoke", func(a testArgs) bool { return a.cc.inputsAreStatic })
	hasOutputs           = predicate("does not have outputs", func(a testArgs) bool { return a.t.NumOut() != 0 })
	mustNotMemoize       = predicate("is marked Memoized", func(a testArgs) bool { return !a.fm.memoize })
	markedMemoized       = predicate("is not marked Memoized", func(a testArgs) bool { return a.fm.memoize })
	markedCacheable      = predicate("is not marked Cacheable", func(a testArgs) bool { return a.fm.cacheable })
	markedSingleton      = predicate("is not marked Singleton", func(a testArgs) bool { return a.fm.singleton })
	notMarkedSingleton   = predicate("is marked Singleton", func(a testArgs) bool { return !a.fm.singleton })
	notMarkedNoCache     = predicate("is marked NotCacheable", func(a testArgs) bool { return !a.fm.notCacheable })
	mappableInputs       = predicate("has inputs that cannot be map keys", func(a testArgs) bool { return mappable(typesIn(a.t)...) })
	possibleMapKey       = predicate("type is not cacheable", func(a testArgs) bool { p, _ := canBeMapKey(typesIn(a.t)); return p })
	returnsTerminalError = predicate("does not return TerminalError", func(a testArgs) bool {
		for _, out := range typesOut(a.t) {
			if out == terminalErrorType {
				return true
			}
		}
		return false
	})
)

var noAnonymousFuncs = predicate("has an untyped functional argument", func(a testArgs) bool {
	return !hasAnonymousFuncs(typesIn(a.t), false) &&
		!hasAnonymousFuncs(typesOut(a.t), false)
})

var noAnonymousExceptFirstInput = predicate("has extra untyped functional arguments", func(a testArgs) bool {
	return !hasAnonymousFuncs(typesIn(a.t), true) &&
		!hasAnonymousFuncs(typesOut(a.t), false)
})

var hasInner = predicate("does not have an Inner function (untyped functional argument in the 1st position)", func(a testArgs) bool {
	t := a.t
	return t.Kind() == reflect.Func && t.NumIn() > 0 && t.In(0).Kind() == reflect.Func
})

var isFuncPointer = predicate("is not a pointer to a function", func(a testArgs) bool {
	t := a.t
	return t.Kind() == reflect.Ptr && t.Elem().Kind() == reflect.Func
})

var invokeRegistry = typeRegistry{
	{
		name: "init func",
		tests: predicates{
			notNil,
			inStatic,
			isFuncPointer,
		},
		mutate: func(a testArgs) {
			a.fm.group = invokeGroup
			a.fm.class = initFunc
			a.fm.flows[outputParams] = toTypeCodes(typesIn(a.t.Elem()))
			a.fm.flows[bypassParams] = toTypeCodes(typesOut(a.t.Elem()))
			a.fm.required = true
			a.fm.isSynthetic = true
		},
	},

	{
		name: "invoke func",
		tests: predicates{
			notNil,
			isFuncPointer,
		},
		mutate: func(a testArgs) {
			a.fm.group = invokeGroup
			a.fm.class = invokeFunc
			a.fm.flows[outputParams] = toTypeCodes(typesIn(a.t.Elem()))
			a.fm.flows[returnedParams] = toTypeCodes(typesOut(a.t.Elem()))
			a.fm.required = true
			a.fm.isSynthetic = true
		},
	},
}

var handlerRegistry = typeRegistry{
	{
		name: "literal value",
		tests: predicates{
			notFunc,
			inStatic,
			notLast,
		},
		mutate: func(a testArgs) {
			a.fm.group = literalGroup
			a.fm.class = literalValue
			// the cast is safe because when the value is a Reflective, we look like
			// like a func and this code only runs for non-funcs.
			a.fm.flows[outputParams] = toTypeCodes([]reflect.Type{a.t.(reflect.Type)})
		},
	},

	{
		name: "fallable singleton injector",
		tests: predicates{
			markedSingleton,
			isFunc,
			inStatic,
			markedCacheable,
			noAnonymousFuncs,
			returnsTerminalError,
			notLast,
			mappableInputs,
			notMarkedNoCache,
			mustNotMemoize,
		},
		mutate: func(a testArgs) {
			a.fm.group = staticGroup
			a.fm.class = fallibleStaticInjectorFunc
			a.fm.flows[inputParams] = toTypeCodes(typesIn(a.t))
			a.fm.flows[outputParams] = toTypeCodes(remapTerminalError(typesOut(a.t)))
		},
	},

	{
		name: "singleton injector",
		tests: predicates{
			markedSingleton,
			isFunc,
			inStatic,
			markedCacheable,
			noAnonymousFuncs,
			notLast,
			mappableInputs,
			notMarkedNoCache,
			mustNotMemoize,
		},
		mutate: func(a testArgs) {
			a.fm.group = staticGroup
			a.fm.class = staticInjectorFunc
			a.fm.flows[inputParams] = toTypeCodes(typesIn(a.t))
			a.fm.flows[outputParams] = toTypeCodes(remapTerminalError(typesOut(a.t)))
		},
	},

	{
		name: "fallible memoized static injector",
		tests: predicates{
			markedMemoized,
			isFunc,
			inStatic,
			markedCacheable,
			noAnonymousFuncs,
			returnsTerminalError,
			notLast,
			mappableInputs,
			notMarkedNoCache,
			possibleMapKey,
			notMarkedSingleton,
		},
		mutate: func(a testArgs) {
			a.fm.group = staticGroup
			a.fm.class = fallibleStaticInjectorFunc
			a.fm.flows[inputParams] = toTypeCodes(typesIn(a.t))
			a.fm.flows[outputParams] = toTypeCodes(remapTerminalError(typesOut(a.t)))
			a.fm.memoized = true
			_, a.fm.mapKeyCheck = canBeMapKey(typesIn(a.t))
		},
	},

	{
		name: "static memoized injector",
		tests: predicates{
			isFunc,
			markedMemoized,
			markedCacheable,
			inStatic,
			notLast,
			hasOutputs,
			mappableInputs,
			noAnonymousFuncs,
			notMarkedNoCache,
			possibleMapKey,
			notMarkedSingleton,
		},
		mutate: func(a testArgs) {
			a.fm.group = staticGroup
			a.fm.class = staticInjectorFunc
			a.fm.flows[inputParams] = toTypeCodes(typesIn(a.t))
			a.fm.flows[outputParams] = toTypeCodes(typesOut(a.t))
			a.fm.memoized = true
			_, a.fm.mapKeyCheck = canBeMapKey(typesIn(a.t))
		},
	},

	{
		name: "fallible static injector",
		tests: predicates{
			isFunc,
			returnsTerminalError,
			markedCacheable,
			inStatic,
			notLast,
			noAnonymousFuncs,
			mustNotMemoize,
			notMarkedNoCache,
			notMarkedSingleton,
		},
		mutate: func(a testArgs) {
			a.fm.group = staticGroup
			a.fm.class = fallibleStaticInjectorFunc
			a.fm.flows[inputParams] = toTypeCodes(typesIn(a.t))
			a.fm.flows[outputParams] = toTypeCodes(remapTerminalError(typesOut(a.t)))
			_, a.fm.mapKeyCheck = canBeMapKey(typesIn(a.t))
		},
	},

	{
		name: "fallible injector",
		tests: predicates{
			isFunc,
			noAnonymousFuncs,
			returnsTerminalError,
			notLast,
			mustNotMemoize,
			unstaticOkay,
			notMarkedSingleton,
		},
		mutate: func(a testArgs) {
			a.fm.group = runGroup
			a.fm.class = fallibleInjectorFunc
			a.fm.flows[inputParams] = toTypeCodes(typesIn(a.t))
			a.fm.flows[outputParams] = toTypeCodes(redactTerminalError(typesOut(a.t)))
			a.fm.flows[returnParams] = toTypeCodes([]reflect.Type{errorType})
		},
	},

	{
		name: "static injector",
		tests: predicates{
			isFunc,
			markedCacheable,
			inStatic,
			noAnonymousFuncs,
			notLast,
			hasOutputs,
			mustNotMemoize,
			notMarkedNoCache,
			notMarkedSingleton,
		},
		mutate: func(a testArgs) {
			a.fm.group = staticGroup
			a.fm.class = staticInjectorFunc
			a.fm.flows[inputParams] = toTypeCodes(typesIn(a.t))
			a.fm.flows[outputParams] = toTypeCodes(typesOut(a.t))
		},
	},

	{
		name: "injector",
		tests: predicates{
			isFunc,
			noAnonymousFuncs,
			notLast,
			mustNotMemoize,
			unstaticOkay,
			notMarkedSingleton,
		},
		mutate: func(a testArgs) {
			a.fm.group = runGroup
			a.fm.class = injectorFunc
			a.fm.flows[inputParams] = toTypeCodes(typesIn(a.t))
			a.fm.flows[outputParams] = toTypeCodes(typesOut(a.t))
		},
	},

	{
		name: "middleware/wrapper",
		tests: predicates{
			isFunc,
			hasInner,
			noAnonymousExceptFirstInput,
			notLast,
			mustNotMemoize,
			unstaticOkay,
			notMarkedSingleton,
		},
		mutate: func(a testArgs) {
			in := typesIn(a.t)
			in[0] = reflect.TypeOf(noTypeExampleValue)
			a.fm.group = runGroup
			a.fm.class = wrapperFunc
			a.fm.flows[inputParams] = toTypeCodes(in)
			a.fm.flows[outputParams] = toTypeCodes(typesIn(a.t.In(0)))
			a.fm.flows[returnParams] = toTypeCodes(typesOut(a.t))
			a.fm.flows[returnedParams] = toTypeCodes(typesOut(a.t.In(0)))
		},
	},

	{
		name: "final/last/endpoint func",
		tests: predicates{
			isFunc,
			isLast,
			noAnonymousFuncs,
			mustNotMemoize,
			unstaticOkay,
			notMarkedSingleton,
		},
		mutate: func(a testArgs) {
			a.fm.group = finalGroup
			a.fm.class = finalFunc
			a.fm.flows[inputParams] = toTypeCodes(typesIn(a.t))
			a.fm.flows[returnParams] = toTypeCodes(typesOut(a.t))
			a.fm.required = true
		},
	},
}

// characterizeFuncDetails returns an annotated copy of the incoming *provider.
func (reg typeRegistry) characterizeFuncDetails(fm *provider, cc charContext) (*provider, error) {
	var rejectReasons []string
	var a testArgs
	if r, ok := fm.fn.(Reflective); ok {
		a = testArgs{
			fm:    fm.copy(),
			t:     reflectiveWrapper{r},
			isNil: false,
			cc:    cc,
		}
	} else {
		v := reflect.ValueOf(fm.fn)
		var isNil bool
		// nolint:exhaustive
		switch v.Type().Kind() {
		case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
			isNil = v.IsNil()
		}
		a = testArgs{
			fm:    fm.copy(),
			t:     v.Type(),
			cc:    cc,
			isNil: isNil,
		}
	}

Match:
	for _, match := range reg {
		for _, predicate := range match.tests {
			if !predicate.test(a) {
				rejectReasons = append(rejectReasons, fmt.Sprintf("%s: %s", match.name, predicate.message))
				continue Match
			}
		}
		a.fm.upRmap = make(map[typeCode]typeCode)
		a.fm.downRmap = make(map[typeCode]typeCode)
		a.fm.flows = make(flowMapType)
		match.mutate(a)
		return a.fm, nil
	}

	// panic(fmt.Sprintf("%s: %s - %s", fm.describe(), t, strings.Join(rejectReasons, "; ")))
	return nil, fm.errorf("Could not type %s to any prototype: %s", a.t, strings.Join(rejectReasons, "; "))
}

func characterizeInitInvoke(fm *provider, context charContext) (*provider, error) {
	return invokeRegistry.characterizeFuncDetails(fm, context)
}

func characterizeFunc(fm *provider, context charContext) (*provider, error) {
	return handlerRegistry.characterizeFuncDetails(fm, context)
}
