//go:generate stringer -type=groupType,classType,flowType -linecomment -output stringer_generated.go
package nject

// TODO: switch flowType, groupType, classType to ints and define a Stringer
// perhaps using generate so that they pretty print.

// Injectors must be identified.  This file defines the characteristics of the
// types to match against.

import (
	"reflect"
)

// TerminalError is a standard error interface.  For fallible injectors,
// TerminalError must be one of the return values.
//
// A non-nil return value terminates the handler call chain.  The
// TerminalError return value gets converted to a regular error value (type=error)
// and (like other return values) it must be consumed by an upstream handler
// or the invoke function. Essentially marking an error return as a TerminalError
// causes special behavior but the effective type is just error.
//
// Functions that return just TerminalError count as having no outputs and
// thus they are treated as specially required if they're in the RUN set.
//
// Note: wrapper functions should not return TerminalError because such
// a return value would not be automatically converted into a regular error.
type TerminalError interface {
	error
}

// Debugging is provided to help diagnose injection issues. *Debugging
// is injected into every chain that consumes it.  Injecting debugging
// into any change can slow down the processing of all other chains because
// debugging is controlled with a global.
type Debugging struct {
	// Included is a list of the providers included in the chain.
	//
	// The format is:
	// "${groupName} ${className} ${providerNameShape}"
	Included []string

	// NamesIncluded is a list of the providers included in the chain.
	// The format is:
	// "${providerName}
	NamesIncluded []string

	// IncludeExclude is a list of all of the providers supplied to
	// create the chain.  Why each was included or not explained.
	// "INCLUDED ${groupName} ${className} ${providerNameShape} BECAUSE ${whyProviderWasInclude}"
	// "EXCLUDED ${groupName} ${className} ${providerNameShape} BECAUSE ${whyProviderWasExcluded}"
	IncludeExclude []string

	// Trace is an nject internal debugging trace that details the
	// decision process to decide which providers are included in the
	// chain.
	Trace string

	// Reproduce is a Go source string that attempts to somewhat anonymize
	// a provider chain as a unit test.  This output is nearly runnable
	// code.  It may need a bit of customization to fully capture a situation.
	Reproduce string

	// Outer is only present within chains generated with Branch().  It is a reference
	// to the Debugging from the main (or outer) injection chain
	Outer *Debugging
}

// Unused is a special type: as an input, it will be provided automatically. If output from a
// MustConsume provider, no consumer is needed. If an injector only returns Unused, then that
// injector will be included in the chain, if possible, same as an injector that doesn't return
// anything at all.
type Unused struct{}

type classType int

const (
	unsetClassType             classType = iota // ?
	fallibleInjectorFunc                        // fallible-injector
	fallibleStaticInjectorFunc                  // fallible-static-injector
	injectorFunc                                // injector
	wrapperFunc                                 // wrapper-func
	finalFunc                                   // final-func
	staticInjectorFunc                          // static-injector
	literalValue                                // literal-value
	initFunc                                    // init-func
	invokeFunc                                  // invoke-func
)

type groupType int

const (
	invokeGroup  groupType = iota // invoke
	literalGroup                  // literal
	staticGroup                   // static
	runGroup                      // run
	finalGroup                    // final
)

type flowType int

const (
	// going up
	returnParams flowType = iota // returns
	// going down
	outputParams // outputs
	// received from above
	inputParams // inputs
	// received from below (callee returned)
	receivedParams // received
	// gathered from the end of the static chain and returned from init
	bypassParams // bypass
	//
	lastFlowType // UNUSED
)

var (
	terminalErrorType = reflect.TypeOf((*TerminalError)(nil)).Elem()

	errorType = reflect.TypeOf((*error)(nil)).Elem()

	ignoreType = reflect.TypeOf((*ignore)(nil)).Elem()

	emptyInterfaceType = reflect.TypeOf((*any)(nil)).Elem()

	debuggingType   = reflect.TypeOf((**Debugging)(nil)).Elem()
	bypassDebugType = reflect.TypeOf((**bypassDebug)(nil)).Elem()

	reflectiveFuncType = reflect.TypeOf((*func([]reflect.Type) []reflect.Type)(nil)).Elem()

	unusedType = reflect.TypeOf((*Unused)(nil)).Elem()
)
