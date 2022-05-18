package nject

import (
	"fmt"
	"reflect"
)

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
// Experimental: the return type of Condense could change in a
// future version to be a *Collection instead so as to support
// better filling of *Debugging.  Since *Collection supports a
// superset of the Provider interface, this should not matter.
func (c *Collection) Condense() (Provider, error) {
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
	return newThing(invokeF.thinReflective), nil
}

// MustCondense panics if Condense fails
func (c *Collection) MustCondense() Provider {
	p, err := c.Condense()
	if err != nil {
		panic(err)
	}
	return p
}
