package nject

// TODO: switch flowType, groupType, classType to ints and define a Stringer
// perhaps using generate so that they pretty print.

// Injectors must be identified.  This file defines the characteristics of the
// types to match against.

import (
	"reflect"
)

// TerminalError is a standard error interface.  For fallible injectors,
// TerminalError must one of the return values.
//
// A non-nil return value terminates the handler call chain.  The
// TerminalError return value gets converted to a regular error value
// and (like other return values) it must be consumed by an upstream handler
// or the invoke function.
type TerminalError interface {
	error
}

// Debugging is provided to help diagnose injection issues. *Debugging
// is injected into every chain that consumes it.
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
	// ceate the chain.  Why each was included or not explained.
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
}

type classType string

const (
	fallibleInjectorFunc       classType = "fallible-injector"
	fallibleStaticInjectorFunc classType = "fallible-static-injector"
	injectorFunc               classType = "injector"
	wrapperFunc                classType = "wrapper-func"
	finalFunc                  classType = "final-func"
	staticInjectorFunc         classType = "static-injector"
	literalValue               classType = "literal-value"
	initFunc                   classType = "init-func"
	invokeFunc                 classType = "invoke-func"
)

type groupType string

const (
	literalGroup groupType = "literal"
	staticGroup  groupType = "static"
	runGroup     groupType = "run"
	finalGroup   groupType = "final"
	invokeGroup  groupType = "invoke"
)

type flowType string

const (
	returnParams   flowType = "returns"  // going up
	outputParams   flowType = "outputs"  // going down
	inputParams    flowType = "inputs"   // recevied from above
	returnedParams flowType = "returned" // received from below
	bypassParams   flowType = "bypass"   // returned from init
)

var terminalErrorType = reflect.TypeOf((*TerminalError)(nil)).Elem()

var errorType = reflect.TypeOf((*error)(nil)).Elem()
