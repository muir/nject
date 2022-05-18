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
// If error is returned, it is tranformed into TerminalError.
// If you want to capture an actual error without aborting the
// parent chan, then embed it in something else.
//
// The condensed provider is bound with Collection.Bind() at
// the time that Condense() is called.
func (c *Collection) Condense() (Provider, error) {
	name := c.name
	if len(c.contents) == 0 {
		return c, nil
	}
	last := c.contents[len(c.contents)-1]
	last.required = true
	lastType := reflect.TypeOf(last.fn)
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
		// collections don't have a convienent method for
		// prepending something to their contents so we'll
		// build a replacement instead.
		c = Sequence(name+"-debug",
			func(d *Debugging, b *bypassDebug) {
				d.Outer = (*Debugging)(b)
			}).Append(name+"-condensed", c)
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

	for i, t := range upOut {
		if t == errorType {
			// we're being a bit naughty here: we modify
			// a slice that's already part of another
			// structure and has been passed to functions
			upOut[i] = terminalErrorType
		}
	}

	bound := newThing(invokeF.thinReflective)

	if !debugFound {
		return bound, nil
	}

	return Sequence(name+"-dbundle",
		func(d *Debugging) *bypassDebug {
			return (*bypassDebug)(d)
		}).Append(name+"-bound", bound), nil
}

// MustCondense panics if Condense fails
func (c *Collection) MustCondense() Provider {
	p, err := c.Condense()
	if err != nil {
		panic(err)
	}
	return p
}
