package nject

// TODO: switch from typeCode to reflect.Type

import (
	"reflect"
	"sync"
)

type typeCode int

var typeCounter = 0
var lock sync.Mutex
var typeMap = make(map[reflect.Type]typeCode)
var reverseMap = make(map[typeCode]reflect.Type)

type noType bool

const noTypeExampleValue noType = false

var noTypeCode = getTypeCode(noTypeExampleValue)

// getTypeCode maps Go types to integers.  It is exported for
// testing purposes.
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
	return tc.Type().String()
}
