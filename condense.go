package nject

import (
	"fmt"
	"reflect"
)

type bypassDebug Debugging

// Condense transforms a collection into a single provider.
// The inputs to the provider are what's
// required for the last function in the Collection to be invoked
// given the rest of the Collection.
//
// At this time, the last function in the collection may not
// be a wrap function.  Wrap functions within the condensed
// collection only wrap the rest of the functions within the
// condensed collection.
//
// All types returned by the last function in the collection or
// or returned by wrap functions are returned by the condensed
// provider.
//
// The condensed provider is bound with Collection.Bind() at
// the time that Condense() is called.
//
// If treatErrorAsTerminal is true then a returned error will be
// treated as a TerminalError. Otherwise it is treated as a
// a regular error being provided into the downward chain.
func (c *Collection) Condense(treatErrorAsTerminal bool) (Provider, error) {
	name := c.name + "-condensed"
	if len(c.contents) == 0 {
		return c, nil
	}
	last := c.contents[len(c.contents)-1]
	last.required = true
	lastType := reflect.TypeOf(last.fn)
	if isWrapper(lastType, last.fn) {
		return nil, fmt.Errorf("Condense cannot operate on collections whose last element is a wrap function")
	}

	// Annotate providers so that the last provider in the collection will
	// be treated as a final function and it's return values will be
	// upOut upflows.
	{
		nonStaticTypes := make(map[typeCode]bool)
		beforeInvoke, afterInvoke, err := c.characterizeAndFlatten(nonStaticTypes)
		if err != nil {
			return nil, err
		}
		ia := make([]any, 0, len(beforeInvoke)+len(afterInvoke))
		for _, fm := range beforeInvoke {
			ia = append(ia, fm)
		}
		for _, fm := range afterInvoke {
			ia = append(ia, fm)
		}
		c = Sequence(name, ia...)
	}

	downIn, _ := c.DownFlows()
	_, upOut := c.UpFlows()

	// If we've got debugging going on inside the condensed collection, let's
	// pipe in the debugging from the outer collection too.  To do that, we
	// use a second type as an extra thing that we want to receive.  In any
	// case, we don't want to directly expose that we want to receive a
	// *Debugging since it is always filled magically by Bind()
	var debugFound bool
	for i, t := range downIn {
		if t != debuggingType {
			continue
		}
		downIn[i] = bypassDebugType
		if debugFound {
			continue
		}
		debugFound = true

		// collections don't have a convenient method for
		// prepending something to their contents so we'll
		// build a replacement instead.
		c = Sequence(name,
			func(d *Debugging, b *bypassDebug) {
				d.Outer = (*Debugging)(b)
			}, c)
	}

	invokeF := &reflectiveBinder{
		thinReflective: thinReflective{
			thinReflectiveArgs: thinReflectiveArgs{
				inputs:  downIn,
				outputs: upOut,
			},
		},
	}

	err := c.Bind(invokeF, nil)
	if err != nil {
		return nil, err
	}

	if treatErrorAsTerminal {
		for i, t := range upOut {
			if t == errorType {
				// we're being a bit naughty here: we modify
				// a slice that's already part of another
				// structure and has been passed to functions
				upOut[i] = terminalErrorType
			}
		}
	}

	bound := newThing(invokeF.thinReflective)

	if !debugFound {
		return bound, nil
	}

	return Sequence(name,
		func(d *Debugging) *bypassDebug {
			return (*bypassDebug)(d)
		},
		bound,
	), nil
}

// MustCondense panics if Condense fails
func (c *Collection) MustCondense(treatErrorAsTerminal bool) Provider {
	p, err := c.Condense(treatErrorAsTerminal)
	if err != nil {
		panic(err)
	}
	return p
}
