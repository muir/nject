package nject

// TODO: add unit test

import (
	"reflect"
	"sync"
)

type in3 [3]interface{}
type in10 [10]interface{}
type in30 [30]interface{}

type cacherFunc func(in []reflect.Value) []reflect.Value

var cachers = make(map[int32]cacherFunc)
var lockLock sync.RWMutex

func generateCache(id int32, fv reflect.Value, l int) cacherFunc {
	lockLock.Lock()
	defer lockLock.Unlock()
	if cacher, ok := cachers[id]; ok {
		return cacher
	}

	cacher := defineCacher(id, fv, l)
	cachers[id] = cacher
	return cacher
}

func interfaceOkay(in []reflect.Value) bool {
	for _, input := range in {
		if !input.CanInterface() {
			return false
		}
	}
	return true
}

func defineCacher(id int32, fv reflect.Value, l int) cacherFunc {
	var lock sync.Mutex

	switch {
	case l <= 3:
		cache := make(map[in3][]reflect.Value)
		return func(in []reflect.Value) []reflect.Value {
			if !interfaceOkay(in) {
				return fv.Call(in)
			}
			lock.Lock()
			defer lock.Unlock()
			var key in3
			for i, v := range in {
				key[i] = v.Interface()
			}
			for i := len(in); i < 3; i++ {
				key[i] = ""
			}
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
			if !interfaceOkay(in) {
				return fv.Call(in)
			}
			lock.Lock()
			defer lock.Unlock()
			var key in10
			for i, v := range in {
				key[i] = v.Interface()
			}
			for i := len(in); i < 10; i++ {
				key[i] = ""
			}
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
			if !interfaceOkay(in) {
				return fv.Call(in)
			}
			lock.Lock()
			defer lock.Unlock()
			var key in30
			for i, v := range in {
				key[i] = v.Interface()
			}
			for i := len(in); i < 30; i++ {
				key[i] = ""
			}
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
