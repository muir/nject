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
	cacheable           bool
	mustCache           bool
	required            bool
	callsInner          bool
	memoize             bool
	loose               bool
	desired             bool
	notCacheable        bool
	mustConsume         bool
	consumptionOptional bool

	// added by characterize
	memoized    bool
	class       classType
	group       groupType
	flows       flowMapType
	isSynthetic bool

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
	upVmapCount                int
	downVmapCount              int

	wrapWrapper          func(valueCollection, func(valueCollection) valueCollection) valueCollection // added in generate
	wrapStaticInjector   func(valueCollection) error                                                  // added in generate
	wrapFallibleInjector func(valueCollection) (bool, valueCollection)                                // added in generate
	wrapEndpoint         func(valueCollection) valueCollection                                        // added in generate
}

// copy does not copy wrappers or flows.
func (fm *provider) copy() *provider {
	if fm == nil {
		return nil
	}
	return &provider{
		origin:              fm.origin,
		index:               fm.index,
		fn:                  fm.fn,
		id:                  fm.id,
		cacheable:           fm.cacheable,
		mustCache:           fm.mustCache,
		required:            fm.required,
		callsInner:          fm.callsInner,
		memoize:             fm.memoize,
		loose:               fm.loose,
		memoized:            fm.memoized,
		desired:             fm.desired,
		mustConsume:         fm.mustConsume,
		consumptionOptional: fm.consumptionOptional,
		notCacheable:        fm.notCacheable,
		class:               fm.class,
		group:               fm.group,
		flows:               fm.flows,
	}
}

type thing interface {
	modify(func(*provider)) thing
	flatten() []*provider
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

	for ii, fm := range c.contents {
		cc := charContext{
			isLast:          ii == len(c.contents)-1,
			inputsAreStatic: true,
		}
		fm, err := characterizeFunc(fm, cc)
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
		switch fm.group {
		case runGroup, invokeGroup:
			for _, out := range fm.flows[outputParams] {
				nonStaticTypes[out] = true
			}
		}

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

func (p provider) renameIfEmpty(i int, name string) *provider {
	if p.origin == "" {
		fm := p.copy()
		fm.origin = name
		if fm.index == -1 {
			fm.index = i
		}
		return fm
	}
	return &p
}

func (p provider) flatten() []*provider {
	return []*provider{&p}
}

func (c Collection) flatten() []*provider {
	return c.contents
}

func (p provider) modify(f func(*provider)) thing {
	fm := p.copy()
	f(fm)
	return fm
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
