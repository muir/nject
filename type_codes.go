package nject

// TODO: switch from typeCode to reflect.Type -- duplicate detection would be lost

import (
	"reflect"
	"sync"

	"github.com/muir/reflectutils"
)

type (
	typeCode  int
	typeCodes []typeCode
)

var (
	typeCounter = 0
	lock        sync.Mutex
	typeMap     = make(map[reflect.Type]typeCode)
	reverseMap  = make(map[typeCode]reflect.Type)
)

type noType bool

const noTypeExampleValue noType = false

var noTypeCode = getTypeCode(noTypeExampleValue)

var unusedTypeCode = getTypeCode(unusedType)

// noNoType filters out noTypeCode from an array of typeCode
func noNoType(types []typeCode) []typeCode {
	found := -1
	for i, t := range types {
		if t == noTypeCode {
			found = i
			break
		}
	}
	if found == -1 {
		return types
	}
	n := make([]typeCode, found, len(types)-1)
	copy(n, types[0:found])
	for i := found + 1; i < len(types); i++ {
		if types[i] != noTypeCode {
			n = append(n, types[i])
		}
	}
	return n
}

// getTypeCode maps reflect.Type to integers.
func getTypeCode(a interface{}) typeCode {
	if a == nil {
		panic("nil has no type")
	}
	t, isType := a.(reflect.Type)
	if !isType {
		t = reflect.TypeOf(a)
	}
	lock.Lock()
	defer lock.Unlock()
	if tc, found := typeMap[t]; found {
		return tc
	}
	typeCounter++
	tc := typeCode(typeCounter)
	typeMap[t] = tc
	reverseMap[tc] = t
	return tc
}

// Type returns the reflect.Type for this typeCode
func (tc typeCode) Type() reflect.Type {
	lock.Lock()
	defer lock.Unlock()
	return reverseMap[tc]
}

// Type returns the reflect.Type for this typeCode
func (tc typeCode) String() string {
	return reflectutils.TypeName(tc.Type())
}

func (tcs typeCodes) Types() []reflect.Type {
	if tcs == nil {
		return nil
	}
	if len(tcs) == 0 {
		return []reflect.Type{}
	}
	lock.Lock()
	defer lock.Unlock()
	t := make([]reflect.Type, len(tcs))
	for i, tc := range tcs {
		t[i] = reverseMap[tc]
	}
	return t
}
