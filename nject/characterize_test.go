package nject

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const lastT = true
const lastF = false
const staticT = true
const staticF = false
const panicT = true
const panicF = false

type intType3 int

var tsiwfsFunc1a = func() intType3 { return 9 }
var tsiwfsFunc2a = func(w http.ResponseWriter, r *http.Request) {}

type interfaceI interface {
	I() int
}
type interfaceJ interface {
	I() int
}
type interfaceK interface {
	I() int
}
type doesI struct {
	i int
}

func (di *doesI) I() int { return di.i * 2 }

type doesJ struct {
	j int
}

func (dj *doesJ) I() int { return dj.j * 3 }

func params() flowMapType {
	return make(flowMapType)
}
func (flows flowMapType) returns(f ...typeCode) flowMapType {
	flows[returnParams] = f
	return flows
}
func (flows flowMapType) input(f ...typeCode) flowMapType {
	flows[inputParams] = f
	return flows
}
func (flows flowMapType) output(f ...typeCode) flowMapType {
	flows[outputParams] = f
	return flows
}
func (flows flowMapType) returned(f ...typeCode) flowMapType {
	flows[returnedParams] = f
	return flows
}
func (flows flowMapType) bypass(f ...typeCode) flowMapType {
	flows[bypassParams] = f
	return flows
}

var characterizeTests = []struct {
	name            string
	fn              interface{}
	expectedClass   classType
	last            bool
	inputsAreStatic bool
	expectedToError bool
	flows           flowMapType
}{
	{
		"invoke: int, string",
		&takesTwoReturnsThree,
		invokeFunc,
		lastF, staticF, panicF,
		params().output(intTC, stringTC).returned(boolTC, stringTC, errorTC),
	},
	{
		"init: int, string",
		&takesTwoReturnsThree,
		initFunc,
		lastF, staticT, panicF,
		params().output(intTC, stringTC).bypass(boolTC, stringTC, errorTC),
	},
	{
		"tsiwfsFunc1a",
		Cacheable(tsiwfsFunc1a),
		staticInjectorFunc,
		lastF, staticT, panicF,
		params().output(intType3TC),
	},
	{
		"tsiwfsFunc2a",
		tsiwfsFunc2a,
		finalFunc,
		lastT, staticT, panicF,
		params().input(responseWriterTC, requestTC),
	},
	{
		"tsiwfsFunc2b",
		Provide("tsiwfsFunc2b", tsiwfsFunc2a),
		finalFunc,
		lastT, staticT, panicF,
		params().input(responseWriterTC, requestTC),
	},
	{
		"static injector",
		Cacheable(Provide("static injector", func(int) string { return "" })),
		staticInjectorFunc,
		lastF, staticT, panicF,
		params().input(intTC).output(stringTC),
	},
	{
		"cacheable not cacheable",
		Cacheable(NotCacheable(Provide("static injector", func(int) string { return "" }))),
		injectorFunc,
		lastF, staticT, panicF,
		params().input(intTC).output(stringTC),
	},
	{
		"mustCache not cacheable",
		MustCache(NotCacheable(Provide("static injector", func(int) string { return "" }))),
		injectorFunc,
		lastF, staticT, panicT,
		params().input(intTC).output(stringTC),
	},
	{
		"injector (not cacheable)",
		func(int) string { return "" },
		injectorFunc,
		lastF, staticT, panicF,
		params().input(intTC).output(stringTC),
	},
	{
		"static injector must cache",
		MustCache(Provide("injector must cache", func(int) string { return "" })),
		staticInjectorFunc,
		lastF, staticF, panicT,
		params().input(intTC).output(stringTC),
	},
	{
		"fallible injector (not cacheable)",
		func(int) string { return "" },
		injectorFunc,
		lastF, staticT, panicF,
		params().input(intTC).output(stringTC),
	},
	{
		"fallible static injector must cache",
		MustCache(Provide("injector must cache", func(int) (string, TerminalError) { return "", nil })),
		fallibleStaticInjectorFunc,
		lastF, staticT, panicF,
		params().input(intTC).output(stringTC, errorTC),
	},
	{
		"fallible injector must cache (panics)",
		MustCache(Provide("injector must cache", func(int) (string, TerminalError) { return "", nil })),
		fallibleStaticInjectorFunc,
		lastF, staticF, panicT,
		params().input(intTC).output(stringTC),
	},
	{
		"minimal injector",
		func() {},
		injectorFunc,
		lastF, staticF, panicF,
		nil,
	},
	{
		"cacheable notCacheable",
		Cacheable(NotCacheable(func() {})),
		injectorFunc,
		lastF, staticT, panicF,
		nil,
	},
	{
		"cacheable notCacheable",
		Cacheable(NotCacheable(func() {})),
		injectorFunc,
		lastF, staticF, panicF,
		nil,
	},
	{
		"minimal fallible injector",
		Cacheable(func() TerminalError { return nil }),
		fallibleInjectorFunc,
		lastF, staticF, panicF,
		params().returns(errorTC),
	},
	{
		"minimal fallible static injector",
		Cacheable(func() TerminalError { return nil }),
		fallibleStaticInjectorFunc,
		lastF, staticT, panicF,
		params().output(errorTC),
	},
	{
		"fallible injector",
		Cacheable(func(int, string) (TerminalError, string) { return nil, "" }),
		fallibleInjectorFunc,
		lastF, staticF, panicF,
		params().
			input(intTC, stringTC).
			output(stringTC).
			returns(errorTC),
	},
	{
		"endpoint (final)",
		func(int, string) string { return "" },
		finalFunc,
		lastT, staticF, panicF,
		params().input(intTC, stringTC).returns(stringTC),
	},
	{
		"consumer of value",
		func(int, string) {},
		injectorFunc,
		lastF, staticT, panicF,
		params().input(intTC, stringTC),
	},
	{
		"invalid: anonymous func that isn't a wrap",
		func(int) func() { return func() {} },
		finalFunc,
		lastT, staticF, panicT,
		nil,
	},
	{
		"invalid: anonymous func that isn't a wrap #2",
		func(func(), int) {},
		finalFunc,
		lastT, staticF, panicT,
		nil,
	},
	{
		"middleware func",
		func(func(http.ResponseWriter) error, string, bool) (int, error) { return 7, nil },
		wrapperFunc,
		lastF, staticT, panicF,
		params().
			input(noTypeCode, stringTC, boolTC).
			output(responseWriterTC).
			returns(intTC, errorTC).
			returned(errorTC),
	},
	{
		"middleware func -- is past static",
		func(func(intType3) (error, intType3), string, bool) (int, error) { return 7, nil },
		wrapperFunc,
		lastF, staticT, panicF,
		params().
			input(noTypeCode, stringTC, boolTC).
			output(intType3TC).
			returns(intTC, errorTC).
			returned(errorTC, intType3TC),
	},
	{
		"simple middleware regression",
		func(i func() error, w http.ResponseWriter) {},
		wrapperFunc,
		lastF, staticF, panicF,
		params().
			input(noTypeCode, responseWriterTC).
			returned(errorTC),
	},
	{
		"simple final regression",
		func(i func() error, w http.ResponseWriter) {},
		wrapperFunc,
		lastT, staticT, panicT,
		params().input(noTypeCode, responseWriterTC).returned(errorTC),
	},
	{
		"invoke: nada",
		&nadaFunc,
		invokeFunc,
		lastF, staticF, panicF,
		nil,
	},
	{
		"init: nada",
		&nadaFunc,
		initFunc,
		lastF, staticT, panicF,
		nil,
	},
	{
		"literal: int",
		Provide("seven", 7),
		literalValue,
		lastF, staticT, panicF,
		params().output(intTC),
	},
	{
		"literal: string past static",
		"foobar",
		literalValue,
		lastF, staticF, panicT,
		nil,
	},
}

var nadaFunc func()
var takesTwoReturnsThree func(int, string) (bool, string, error)

var boolTC = getTypeCode(reflect.TypeOf((*bool)(nil)).Elem())
var intTC = getTypeCode(reflect.TypeOf((*int)(nil)).Elem())
var intType3TC = getTypeCode(reflect.TypeOf((*intType3)(nil)).Elem())
var stringTC = getTypeCode(reflect.TypeOf((*string)(nil)).Elem())
var requestTC = getTypeCode(reflect.TypeOf((**http.Request)(nil)).Elem())
var responseWriterTC = getTypeCode(reflect.TypeOf((*http.ResponseWriter)(nil)).Elem())
var errorTC = getTypeCode(errorType)

func een(i []typeCode) string {
	var s []string
	for _, c := range i {
		s = append(s, c.Type().String())
	}
	return "[" + strings.Join(s, "; ") + "]"

	// if len(i) == 0 { return nil }
	// return i
}

// This tests the basic functionality of characterizeFunc()
func TestCharacterize(t *testing.T) {
	wrapTest(t, func(t *testing.T) {
		for i, test := range characterizeTests {
			reg := handlerRegistry
			cc := charContext{
				isLast:          test.last,
				inputsAreStatic: test.inputsAreStatic,
			}
			charFunc := characterizeFunc
			doing := ""
			if strings.HasPrefix(test.name, "invoke: ") {
				doing = " (invoke)"
				charFunc = characterizeInitInvoke
				reg = invokeRegistry
			} else if strings.HasPrefix(test.name, "init: ") {
				doing = " (init)"
				charFunc = characterizeInitInvoke
				reg = invokeRegistry
			}
			t.Logf("trying to characterize%s... %s", doing, test.name)
			originFm := newProvider(test.fn, i, test.name)
			fm, err := charFunc(originFm, cc)
			if test.expectedToError {
				assert.Error(t, err, "expected err for"+test.name)
				continue
			} else {
				require.NoError(t, err, "error for "+test.name)
			}
			require.NotNil(t, fm, "fm defined "+test.name)
			if !assert.Equal(t, test.expectedClass, fm.class, "type: "+test.name) {
				for _, match := range reg {
					redactedReg := typeRegistry{match}
					_, err := redactedReg.characterizeFuncDetails(fm, cc)
					if err == nil {
						t.Logf("Is %s? yes", match.name)
					} else {
						t.Logf("Is %s? %s", match.name, err)
					}
				}
			}
			for ft, ev := range test.flows {
				t.Logf("flow %s: %s", ft, een(ev))
				assert.EqualValues(t, een(ev), een(fm.flows[ft]), fmt.Sprintf("%s flow: %s", ft, test.name))
			}
			for ft, gv := range fm.flows {
				if test.flows[ft] == nil {
					t.Logf("flow %s: %s", ft, een(gv))
					assert.EqualValues(t, een(test.flows[ft]), een(gv), fmt.Sprintf("%s flow %s", ft, test.name))
				}
			}
		}
	})
}
