package nject

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
)

var (
	debugLock     sync.RWMutex
	debug         uint32
	debugOutput   string
	debugOutputMu sync.Mutex
)

var (
	debuglnHook func(...interface{})
	debugfHook  func(string, ...interface{})
)

func debugEnabled() bool {
	return atomic.LoadUint32(&debug) == 1
}

func debugln(stuff ...interface{}) {
	if !debugEnabled() {
		return
	}

	debugOutputMu.Lock()
	if debuglnHook != nil {
		debuglnHook(stuff...)
	} else {
		debugOutput += fmt.Sprintln(stuff...)
	}
	debugOutputMu.Unlock()
}

func debugf(format string, stuff ...interface{}) {
	if !debugEnabled() {
		return
	}

	debugOutputMu.Lock()
	if debugfHook != nil {
		debugfHook(format, stuff...)
	} else {
		debugOutput += fmt.Sprintf(format+"\n", stuff...)
	}
	debugOutputMu.Unlock()
}

func captureDoBindDebugging(sc *Collection, invokeF *provider, initF *provider) string {
	debugLock.Lock()
	if atomic.SwapUint32(&debug, 1) == 1 {
		return "already capturing"
	}
	defer func() {
		atomic.StoreUint32(&debug, 0)
		debugLock.Unlock()
	}()

	debugOutputMu.Lock()
	debugOutput = ""
	debugOutputMu.Unlock()

	_ = doBind(sc, invokeF, initF, false)

	funcs := make([]*provider, len(sc.contents))
	for i, f := range sc.contents {
		funcs[i], _ = characterizeFunc(f, charContext{inputsAreStatic: true})
	}
	reproduce := generateReproduce(funcs, invokeF, initF)
	debugOutputMu.Lock()
	debugStr := debugOutput + "\n\n\n" + reproduce
	debugOutputMu.Unlock()
	return debugStr
}

func dumpValueArray(va []reflect.Value, context string, vMap map[typeCode]int) {
	if !debugEnabled() {
		return
	}
	if len(vMap) > 0 {
		reverseMap := make(map[int]reflectType)
		for tc, i := range vMap {
			reverseMap[i] = tc.Type()
		}

		for i, v := range va {
			if v.IsValid() {
				debugf("value at %s: %d: %s: %s: %v", context, i, reverseMap[i], v.Type(), v.Interface())
			} else {
				debugf("value at %s: %d: %s: UNINITIALIZED", context, i, reverseMap[i])
			}
		}
		return
	}
	for i, v := range va {
		if v.IsValid() {
			debugf("value at %s: %d: %s: %v", context, i, v.Type(), v.Interface())
		} else {
			debugf("value at %s: %d: UNINITIALIZED", context, i)
		}
	}
}

/*
func dumpVmap(context string, vMap map[typeCode]int) {
	if !debug {
		return
	}
	out := "value map " + context
	for tc, i := range vMap {
		out += fmt.Sprintf("\n\t%s -> %d", tc.Type(), i)
	}
	debugln(out)

*/

func dumpF(context string, fm *provider) {
	if !debugEnabled() {
		return
	}
	var out string
	out += fmt.Sprintf("%s: ID %d: %s", context, fm.id, fm)
	out += fmt.Sprintf("\n\tclass: %s\n\tgroup: %s", fm.class, fm.group)
	for name, flow := range fm.flows {
		if len(flow) > 0 {
			out += fmt.Sprintf("\n\t%s flow: %s", flowType(name), formatFlow(flow))
		}
	}
	for upDown, rMap := range map[string]map[typeCode]typeCode{
		"up":     fm.upRmap,
		"down":   fm.downRmap,
		"bypass": fm.bypassRmap,
	} {
		if len(rMap) > 0 {
			out += fmt.Sprintf("\n\t%s map:", upDown)
			for from, to := range rMap {
				out += fmt.Sprintf("\n\t\t%s -> %s", from.Type(), to.Type())
			}
		}
	}
	if fm.include {
		out += "\n\tincluded"
	}
	if fm.required {
		out += "\n\trequired"
	} else if fm.wanted {
		out += "\n\twanted"
	} else if fm.desired {
		out += "\n\tdesired"
	}
	if fm.mustConsume {
		out += "\n\tmust consume"
	}
	if fm.memoized || fm.memoize {
		out += fmt.Sprintf("\n\tmemoize %v memoized %v", fm.memoize, fm.memoized)
	}
	if fm.cannotInclude != nil {
		out += fmt.Sprintf("\n\tCANNOT: %s", fm.cannotInclude)
	}
	for param, users := range fm.d.usesDetail {
		for tc, dep := range users {
			out += fmt.Sprintf("\n\tUSES: %s (%s %s)", dep, flowType(param), tc)
		}
	}
	for _, dep := range fm.d.usedBy {
		out += fmt.Sprintf("\n\tUSED BY: %s", dep)
	}
	debugln(out)
}

func formatFlow(flow []typeCode) string {
	if !debugEnabled() {
		return ""
	}
	var types []string
	for _, tc := range flow {
		types = append(types, tc.Type().String())
	}
	return strings.Join(types, ", ")
}

func elem(i interface{}) reflect.Type {
	t := reflect.TypeOf(i)
	if t.Kind() == reflect.Ptr {
		return t.Elem()
	}
	return t
}

func generateReproduce(funcs []*provider, invokeF *provider, initF *provider) string {
	subs := make(map[typeCode]string)
	t := ""
	f := "func TestRegression(t *testing.T) {\n"
	f += "\twrapTest(t, func(t *testing.T) {\n"
	f += "\t\tcalled := make(map[string]int)\n"
	f += "\t\tvar invoker " + funcSig(subs, &t, elem(invokeF.fn)) + "\n"
	initName := "nil"
	if initF != nil {
		f += "\t\tvar initer " + funcSig(subs, &t, elem(initF.fn)) + "\n"
		initName = "&initer"
	}
	f += "\t\trequire.NoError(t,\n"
	f += "\t\t\tSequence(\"regression\",\n"

	for _, fm := range funcs {
		if fm == nil {
			continue
		}
		if fm.isSynthetic {
			continue
		}
		if fm.fn == nil {
			continue
		}
		// f += fmt.Sprintf("\t\t\t\t// %s\n", fm)
		f += "\t\t\t\t"
		close := ""
		for annotation, active := range map[string]bool{
			"Cacheable":           fm.cacheable,
			"Required":            fm.required,
			"Desired":             fm.desired,
			"Memoize":             fm.memoize,
			"Loose":               fm.loose,
			"NotCacheable":        fm.notCacheable,
			"MustConsume":         fm.mustConsume,
			"ConsumptionOptional": fm.consumptionOptional,
		} {
			if active {
				f += annotation + "("
				close += ")"
			}
		}
		n := fm.origin
		if fm.index != -1 {
			n = fmt.Sprintf("%s-%d", fm.origin, fm.index)
		}
		f += fmt.Sprintf("Provide(%q, ", n)
		close += ")"
		typ := reflect.TypeOf(fm.fn)
		if typ.Kind() == reflect.Func {
			f += "func("
			skip := 0
			if fm.class == wrapperFunc {
				f += "inner " + funcSig(subs, &t, typ.In(0)) + ", "
				skip = 1
			}
			f += strings.Join(addVarnames(substituteTypes(subs, &t, typesIn(typ)[skip:])), ", ") + ") "
			out := typesOut(typ)
			switch len(out) {
			case 0:
				// nothing
			case 1:
				f += " " + substituteTypes(subs, &t, out)[0]
			default:
				f += " (" + strings.Join(substituteTypes(subs, &t, out), ", ") + ")"
			}
			if fm.class == wrapperFunc {
				f += " {\n"
				f += fmt.Sprintf("\t\t\t\t\tcalled[%q]++\n", n)
				f += "\t\t\t\t\tinner(" + strings.Join(substituteDefaults(subs, typesIn(typ.In(0))), ", ") + ")\n"
				f += "\t\t\t\t\treturn " + strings.Join(substituteDefaults(subs, out), ", ") + "\n"
				f += "\t\t\t\t}"
			} else {
				f += fmt.Sprintf(" { called[%q]++; return %s }", n, strings.Join(substituteDefaults(subs, out), ", "))
			}
			f += close + ","
			if fm.include {
				f += " // included"
			}
			f += "\n"
		} else {
			tca := substituteTypes(subs, &t, []reflect.Type{typ})
			def := substituteDefaults(subs, []reflect.Type{typ})
			f += fmt.Sprintf("%s(%s)%s,\n", tca[0], def[0], close)
		}
	}
	f += "\t\t\t).Bind(&invoker, " + initName + "))\n"
	if initF != nil {
		f += "\t\tiniter(" + strings.Join(substituteDefaults(subs, typesIn(elem(initF.fn))), ", ") + ")\n"
	}
	f += "\t\tinvoker(" + strings.Join(substituteDefaults(subs, typesIn(elem(invokeF.fn))), ", ") + ")\n"
	f += "\t})\n"
	f += "}\n"
	return t + "\n" + f
}

// TODO: take note of which interfaces implement each other and new interfaces that
// follow the same pattern.
func substituteTypes(subs map[typeCode]string, defineTypes *string, types []reflect.Type) []string {
	var replacements []string
	for _, typ := range types {
		tc := getTypeCode(typ)
		if subs[tc] == "" {
			if strings.HasPrefix(typ.String(), "nject.") {
				subs[tc] = strings.TrimPrefix(typ.String(), "nject.")
			} else if strings.HasPrefix(typ.String(), "*nject.") {
				subs[tc] = "*" + strings.TrimPrefix(typ.String(), "*nject.")
			} else if tc == getTypeCode(errorType) {
				subs[tc] = "error"
			} else if typ.Kind() == reflect.Interface {
				if typ.NumMethod() == 0 {
					if typ.Name() == "interface {}" {
						subs[tc] = "interface {}"
					} else {
						subs[tc] = fmt.Sprintf("i%03d", tc)
						*defineTypes += fmt.Sprintf("type i%03d interface{} // %s\n", tc, tc)
					}
				} else {
					subs[tc] = fmt.Sprintf("i%03d", tc)
					*defineTypes += fmt.Sprintf("// %s\ntype i%03d interface{\n\tx%03d()\n}\n", tc, tc, tc)
				}
			} else {
				subs[tc] = fmt.Sprintf("s%03d", tc)
				*defineTypes += fmt.Sprintf("// %s\ntype s%03d int\n", tc, tc)
			}
		}
		replacements = append(replacements, subs[tc])
	}
	return replacements
}

func substituteDefaults(subs map[typeCode]string, types []reflect.Type) []string {
	var def []string
	for _, typ := range types {
		r := subs[getTypeCode(typ)]
		if strings.HasPrefix(r, "i") {
			def = append(def, "nil")
		} else if strings.HasPrefix(r, "s") {
			def = append(def, "0")
		} else if r == "InjectorsDebugging" {
			def = append(def, `""`)
		} else if r == "InjectorsReproduce" {
			def = append(def, `""`)
		} else {
			def = append(def, "nil")
		}
	}
	return def
}

func funcSig(subs map[typeCode]string, defineTypes *string, typ reflect.Type) string {
	f := "func("
	f += strings.Join(substituteTypes(subs, defineTypes, typesIn(typ)), ", ")
	f += ") "
	out := typesOut(typ)
	switch len(out) {
	case 0:
		// nothing
	case 1:
		f += " " + substituteTypes(subs, defineTypes, out)[0]
	default:
		f += " (" + strings.Join(substituteTypes(subs, defineTypes, out), ", ") + ")"
	}
	return f
}

func addVarnames(in []string) []string {
	var out []string
	for _, v := range in {
		out = append(out, fmt.Sprintf("_ %s", v))
	}
	return out
}
