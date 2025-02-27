package nject

import (
	"reflect"
	"sync"
	"unicode"
	"unicode/utf8"
)

type (
	in3  [3]any
	in10 [10]any
	in30 [30]any
	in90 [90]any
)

type cacherFunc func(in []reflect.Value) []reflect.Value

var (
	cachers    = make(map[int32]cacherFunc)
	singletons = make(map[int32]cacherFunc)
	lockLock   sync.RWMutex
)

// canSimpleTypeBeMapKey cannot handle structs or interfaces
func canSimpleTypeBeMapKey(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128, reflect.Array,
		reflect.Ptr, reflect.String:
		return true
	case reflect.Chan, reflect.Func, reflect.Map,
		reflect.Slice, reflect.UnsafePointer:
		return false
	case reflect.Interface, reflect.Struct, reflect.Invalid:
		//nolint:gocritic // could remove case entirely
		fallthrough
	default:
		// we shouldn't be here
		return false
	}
}

func canValueBeMapKey(v reflect.Value, recurseOkay bool) bool {
	if !v.IsValid() {
		// this is actually nil, but we can't do Interface on it so supporting
		// typed nils is too hard
		return false
	}
	//nolint:exhaustive // on purpose
	switch v.Type().Kind() {
	case reflect.Interface:
		if v.IsNil() {
			return true
		}
		if !recurseOkay {
			return false
		}
		// Is this right?
		if !v.CanInterface() {
			return false
		}
		iface := v.Interface()
		value := reflect.ValueOf(iface)
		return canValueBeMapKey(value, false)
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if !canValueBeMapKey(v.Field(i), true) {
				return false
			}
		}
		return true
	default:
		return canSimpleTypeBeMapKey(v.Type())
	}
}

// canBeMapKey examines an array of types.  If any of those types cannot
// be a map key, then it returns false.  If the ability for those types
// to be a map key cannot be determined just by looking at the types, then
// it returns true and also returns a function that given an array of
// values, determines if the values can be map keys.
func canBeMapKey(in []reflect.Type) (bool, func([]reflect.Value) bool) {
	var checkers []func([]reflect.Value) bool
	for i, t := range in {
		i := i
		//nolint:exhaustive // on purpose
		switch t.Kind() {
		case reflect.Struct:
			for j := 0; j < t.NumField(); j++ {
				f := t.Field(j)
				ok, check := canBeMapKey([]reflect.Type{f.Type})
				if !ok {
					return false, nil
				}
				if check != nil {
					r, _ := utf8.DecodeRuneInString(f.Name)
					if unicode.IsLower(r) {
						// If the type doesn't make it clear that the field
						// is acceptable as a map key (eg: it's an interface)
						// then since the field isn't exported, we cannot get
						// the value and thus we cannot use it as part of a
						// map key
						return false, nil
					}
					checkers = append(checkers, func(in []reflect.Value) bool {
						return check([]reflect.Value{in[i].FieldByIndex(f.Index)})
					})
				}
			}
		case reflect.Interface:
			// We cannot determine the map key compatibility of interface types.  They
			// may be okay, or they may not be okay.  The actual type type will have to be
			// checked once we have a value.
			checkers = append(checkers, func(in []reflect.Value) bool {
				return canValueBeMapKey(in[i], true)
			})
		default:
			if !canSimpleTypeBeMapKey(t) {
				return false, nil
			}
		}
	}
	switch len(checkers) {
	case 0:
		return true, nil
	case 1:
		return true, checkers[0]
	default:
		return true, func(in []reflect.Value) bool {
			for _, f := range checkers {
				if !f(in) {
					return false
				}
			}
			return true
		}
	}
}

func generateLookup(fm *provider, fv canCall, numInputs int) cacherFunc {
	if fm.memoized {
		return generateCache(fm.id, fv, numInputs, fm.mapKeyCheck)
	}
	if fm.singleton {
		return generateSingleton(fm.id, fv)
	}
	return nil
}

func generateSingleton(id int32, fv canCall) cacherFunc {
	lockLock.Lock()
	defer lockLock.Unlock()
	if singleton, ok := singletons[id]; ok {
		return singleton
	}

	var once sync.Once
	var out []reflect.Value
	singleton := func(in []reflect.Value) []reflect.Value {
		once.Do(func() {
			out = fv.Call(in)
		})
		return out
	}
	singletons[id] = singleton
	return singleton
}

func generateCache(id int32, fv canCall, l int, okayCheck func([]reflect.Value) bool) cacherFunc {
	lockLock.Lock()
	defer lockLock.Unlock()
	if cacher, ok := cachers[id]; ok {
		return cacher
	}

	cacher := defineCacher(id, fv, l, okayCheck)
	cachers[id] = cacher
	return cacher
}

func fillKeyFromInputs(key []any, in []reflect.Value) {
	for i, v := range in {
		if !v.IsValid() {
			key[i] = ""
			continue
		}
		if v.Type().Kind() == reflect.Interface && v.IsNil() {
			key[i] = ""
			continue
		}
		key[i] = v.Interface()
	}
	for i := len(in); i < len(key); i++ {
		key[i] = ""
	}
}

func defineCacher(_ int32, fv canCall, l int, okayCheck func([]reflect.Value) bool) cacherFunc {
	var lock sync.Mutex

	switch {
	case l <= 3:
		cache := make(map[in3][]reflect.Value)
		return func(in []reflect.Value) []reflect.Value {
			if okayCheck != nil && !okayCheck(in) {
				return fv.Call(in)
			}
			lock.Lock()
			defer lock.Unlock()
			var key in3
			fillKeyFromInputs(key[:], in)
			if out, found := cache[key]; found {
				return out
			}
			out := fv.Call(in)
			cache[key] = out
			return out
		}

	case l <= 10:
		cache := make(map[in10][]reflect.Value)
		return func(in []reflect.Value) []reflect.Value {
			if okayCheck != nil && !okayCheck(in) {
				return fv.Call(in)
			}
			lock.Lock()
			defer lock.Unlock()
			var key in10
			fillKeyFromInputs(key[:], in)
			if out, found := cache[key]; found {
				return out
			}
			out := fv.Call(in)
			cache[key] = out
			return out
		}

	case l <= 30:
		cache := make(map[in30][]reflect.Value)
		return func(in []reflect.Value) []reflect.Value {
			if okayCheck != nil && !okayCheck(in) {
				return fv.Call(in)
			}
			lock.Lock()
			defer lock.Unlock()
			var key in30
			fillKeyFromInputs(key[:], in)
			if out, found := cache[key]; found {
				return out
			}
			out := fv.Call(in)
			cache[key] = out
			return out
		}

	case l <= 90:
		cache := make(map[in90][]reflect.Value)
		return func(in []reflect.Value) []reflect.Value {
			if okayCheck != nil && !okayCheck(in) {
				return fv.Call(in)
			}
			lock.Lock()
			defer lock.Unlock()
			var key in90
			fillKeyFromInputs(key[:], in)
			if out, found := cache[key]; found {
				return out
			}
			out := fv.Call(in)
			cache[key] = out
			return out
		}

	default:
		debugf("number of arguments exceeds maximum!  %d", l)
		return func(in []reflect.Value) []reflect.Value {
			return fv.Call(in)
		}
	}
}
