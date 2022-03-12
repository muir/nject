package nject

import (
	"errors"
	"fmt"
	"reflect"
	"sync/atomic"
)

var idCounter int32

// provider is an annotated reference to a provider
type provider struct {
	origin string
	index  int
	fn     interface{}
	id     int32

	// user annotations
	nonFinal            bool
	cacheable           bool
	mustCache           bool
	required            bool
	callsInner          bool
	memoize             bool
	loose               bool
	desired             bool
	shun                bool
	notCacheable        bool
	mustConsume         bool
	consumptionOptional bool
	singleton           bool
	cluster             int32

	// added by characterize
	memoized    bool
	class       classType
	group       groupType
	flows       flowMapType
	isSynthetic bool
	mapKeyCheck func([]reflect.Value) bool

	// added during include calculations
	cannotInclude error
	wanted        bool
	whyIncluded   string
	upRmap        map[typeCode]typeCode //  overrides types of returned parameters
	downRmap      map[typeCode]typeCode //  overrides types of input parameters
	bypassRmap    map[typeCode]typeCode //  overrides types of returning parameters
	include       bool
	d             includeWorkingData

	// added during binding
	chainPosition              int
	mustZeroIfRemainderSkipped []typeCode
	mustZeroIfInnerNotCalled   []typeCode
	vmapCount                  int

	// added when generating
	wrapWrapper          func(valueCollection, func(valueCollection)) // added in generate
	wrapStaticInjector   func(valueCollection) error                  // added in generate
	wrapFallibleInjector func(valueCollection) bool                   // added in generate
	wrapEndpoint         func(valueCollection)                        // added in generate
}

// copy does not copy wrappers or flows.
// It only copies the base, user annotations, and characterize annotations;
// it does not not copy include calculations, binding, or generating.
func (fm *provider) copy() *provider {
	if fm == nil {
		return nil
	}
	return &provider{
		origin:              fm.origin,
		index:               fm.index,
		fn:                  fm.fn,
		id:                  fm.id,
		nonFinal:            fm.nonFinal,
		cacheable:           fm.cacheable,
		mustCache:           fm.mustCache,
		required:            fm.required,
		callsInner:          fm.callsInner,
		memoize:             fm.memoize,
		loose:               fm.loose,
		desired:             fm.desired,
		shun:                fm.shun,
		notCacheable:        fm.notCacheable,
		mustConsume:         fm.mustConsume,
		consumptionOptional: fm.consumptionOptional,
		singleton:           fm.singleton,
		cluster:             fm.cluster,
		memoized:            fm.memoized,
		class:               fm.class,
		group:               fm.group,
		flows:               fm.flows,
		isSynthetic:         fm.isSynthetic,
		mapKeyCheck:         fm.mapKeyCheck,
	}
}

type thing interface {
	modify(func(*provider)) thing
	flatten() []*provider

	// For single providers, DownFlows includes all inputs and
	// all outputs.  For collections, Downflows only includes
	// the net inputs and net outputs.
	DownFlows() (inputs []reflect.Type, outputs []reflect.Type)

	// For single providers, Upflows includes all consumes and
	// all returns.  For collections, Upflows only includes
	// the net consumes and returns.
	UpFlows() (consume []reflect.Type, produce []reflect.Type)
}

func newThing(fn interface{}) thing {
	switch v := fn.(type) {
	case *provider:
		return v
	case provider:
		return v
	case *Collection:
		return v
	case Collection:
		return v
	default:
		return newProvider(fn, -1, "")
	}
}

func newProvider(fn interface{}, index int, origin string) *provider {
	if fn == nil {
		return nil
	}
	if fm, isFuncO := fn.(*provider); isFuncO {
		return fm.copy()
	}
	if c, isCollection := fn.(*Collection); isCollection {
		if len(c.contents) == 1 {
			return newProvider(c.contents[0], index, origin)
		}
		panic("Cannot turn Collection into a function")
	}
	return &provider{
		origin: origin,
		index:  index,
		fn:     fn,
		id:     atomic.AddInt32(&idCounter, 1),
	}
}

func (fm *provider) String() string {
	var t string
	if fm.fn == nil {
		t = "nil"
	} else {
		t = reflect.TypeOf(fm.fn).String()
	}
	class := ""
	if fm.class != "" {
		class = string(fm.class) + ": "
	}
	if fm.index >= 0 {
		return fmt.Sprintf("%s%s(%d) [%s]", class, fm.origin, fm.index, t)
	}
	return fmt.Sprintf("%s%s [%s]", class, fm.origin, t)
}

func (fm *provider) errorf(format string, args ...interface{}) error {
	return errors.New(fm.String() + ": " + fmt.Sprintf(format, args...))
}

// This characterizes all the providers and flattens the collection into
// a couple of lists of providers: providers that run before invoke; and
// providers that run after invoke.
func (c Collection) characterizeAndFlatten(nonStaticTypes map[typeCode]bool) ([]*provider, []*provider, error) {
	debugln("BEGIN characterizeAndFlatten")
	defer debugln("END characterizeAndFlatten")

	afterInit := make([]*provider, 0, len(c.contents))
	afterInvoke := make([]*provider, 0, len(c.contents))

	c.reorderNonFinal()

	// Handle mutations
	var mutated bool
	for i := 0; i < len(c.contents); i++ {
		fm := c.contents[i]
		g, ok := fm.fn.(generatedFromInjectionChain)
		if !ok {
			continue
		}
		replacement, err := g.ReplaceSelf(
			Collection{
				name:     "before",
				contents: c.contents[:i],
			},
			Collection{
				name:     "after",
				contents: c.contents[i+1:],
			})
		if err != nil {
			return nil, nil, err
		}
		flat := replacement.flatten()
		if len(flat) == 1 {
			c.contents[i] = flat[0]
		} else {
			n := make([]*provider, 0, len(c.contents)+len(flat)-1)
			n = append(n, c.contents[:i]...)
			n = append(n, flat...)
			n = append(n, c.contents[i+1:]...)
			c.contents = n
		}
	}
	if mutated {
		c.reorderNonFinal()
	}

	for ii, fm := range c.contents {
		cc := charContext{
			isLast:          ii == len(c.contents)-1,
			inputsAreStatic: true,
		}
		var err error
		fm, err = characterizeFunc(fm, cc)
		if err != nil {
			return nil, nil, err
		}

		if fm.group == staticGroup {
			for _, in := range fm.flows[inputParams] {
				if nonStaticTypes[in] {
					cc.inputsAreStatic = false
					fm, err = characterizeFunc(fm, cc)
					if err != nil {
						return nil, nil, err
					}
					break
				}
			}
		}
		// nolint:exhaustive
		switch fm.group {
		case runGroup, invokeGroup:
			for _, out := range fm.flows[outputParams] {
				nonStaticTypes[out] = true
			}
		}

		// nolint:exhaustive
		switch fm.group {
		case staticGroup, literalGroup:
			afterInit = append(afterInit, fm)
		case finalGroup, runGroup:
			afterInvoke = append(afterInvoke, fm)
		default:
			return nil, nil, fmt.Errorf("internal error: not expecting %s group", fm.group)
		}
	}
	return afterInit, afterInvoke, nil
}

func newCollection(name string, funcs ...interface{}) *Collection {
	var contents []*provider
	for i, fn := range funcs {
		if fn == nil {
			continue
		}
		switch v := fn.(type) {
		case *Collection:
			if v != nil {
				contents = append(contents, v.contents...)
			}
		case Collection:
			contents = append(contents, v.contents...)
		case *provider:
			if v != nil {
				contents = append(contents, v.renameIfEmpty(i, name))
			}
		case provider:
			contents = append(contents, v.renameIfEmpty(i, name))
		default:
			contents = append(contents, newProvider(fn, i, name))
		}
	}
	return &Collection{
		name:     name,
		contents: contents,
	}
}

func (fm provider) renameIfEmpty(i int, name string) *provider {
	if fm.origin == "" {
		nfm := fm.copy()
		nfm.origin = name
		if nfm.index == -1 {
			nfm.index = i
		}
		return nfm
	}
	return &fm
}

func (fm provider) flatten() []*provider {
	return []*provider{&fm}
}

func (c Collection) flatten() []*provider {
	return c.contents
}

func (fm provider) modify(f func(*provider)) thing {
	nfm := fm.copy()
	f(nfm)
	return nfm
}

func (c Collection) modify(f func(*provider)) thing {
	n := make([]*provider, len(c.contents))
	for i, fm := range c.contents {
		fm = fm.copy()
		f(fm)
		n[i] = fm
	}
	return &Collection{
		name:     c.name,
		contents: n,
	}
}

func (fm provider) DownFlows() ([]reflect.Type, []reflect.Type) {
	if r, ok := fm.fn.(Reflective); ok {
		return effectiveOutputs(reflectiveWrapper{r})
	}
	if _, ok := fm.fn.(generatedFromInjectionChain); ok {
		return nil, nil
	}
	v := reflect.ValueOf(fm.fn)
	if !v.IsValid() {
		return nil, nil
	}
	t := v.Type()
	if t.Kind() == reflect.Func {
		return effectiveOutputs(t)
	}
	return nil, []reflect.Type{t}
}

// The inputs to inner() are additional types that are provided
// downstream.
func effectiveOutputs(fn reflectType) ([]reflect.Type, []reflect.Type) {
	inputs := typesIn(fn)
	outputs := typesOut(fn)
	if len(inputs) == 0 || inputs[0].Kind() != reflect.Func {
		return inputs, outputs
	}
	i0 := inputs[0]
	inputs = inputs[1:]
	return inputs, typesIn(i0)
}

func (c Collection) netFlows(f func(fm *provider) ([]reflect.Type, []reflect.Type)) ([]reflect.Type, []reflect.Type) {
	seenIn := make(map[reflect.Type]struct{})
	uniqueIn := make([]reflect.Type, 0, len(c.contents)*4)
	seenOut := make(map[reflect.Type]struct{})
	uniqueOut := make([]reflect.Type, 0, len(c.contents)*4)
	for _, fm := range c.contents {
		inputs, outputs := f(fm)
		inputsByType := make(map[reflect.Type]struct{})
		for _, input := range inputs {
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

func (fm provider) UpFlows() ([]reflect.Type, []reflect.Type) {
	if r, ok := fm.fn.(Reflective); ok {
		return effectiveReturns(reflectiveWrapper{r})
	}
	if _, ok := fm.fn.(generatedFromInjectionChain); ok {
		return nil, nil
	}
	v := reflect.ValueOf(fm.fn)
	if !v.IsValid() {
		return nil, nil
	}
	t := v.Type()
	if t.Kind() == reflect.Func {
		return effectiveReturns(t)
	}
	return nil, []reflect.Type{t}
}

// Only wrapper functions consume return values and only
// wrapper functions provide return values
func effectiveReturns(fn reflectType) ([]reflect.Type, []reflect.Type) {
	inputs := typesIn(fn)
	if len(inputs) == 0 || inputs[0].Kind() != reflect.Func {
		return nil, nil
	}
	i0 := inputs[0]
	return typesOut(i0), typesOut(fn)
}

// UpFlows provides the net unresolved flows up the injection chain.
// If a type is used both as value it consumes as a return value and also
// as a value that it in turn returns, then the up flow for that provider will
// be counted only by what it consumes.
func (c Collection) UpFlows() ([]reflect.Type, []reflect.Type) {
	return c.netFlows(func(fm *provider) ([]reflect.Type, []reflect.Type) {
		return fm.UpFlows()
	})
}

// Reorder collection to handle providers marked nonFinal by shifting
// the last provider that isn't marked nonFinal to the end of the slice.
func (c Collection) reorderNonFinal() {
	for i := len(c.contents) - 1; i >= 0; i-- {
		fm := c.contents[i]
		if fm.nonFinal {
			continue
		}
		if i == len(c.contents)-1 {
			// no re-ordering required
			return
		}
		final := c.contents[i]
		for j := i; j < len(c.contents)-1; j++ {
			c.contents[j] = c.contents[j+1]
		}
		c.contents[len(c.contents)-1] = final
		return
	}
}
