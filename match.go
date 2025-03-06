package nject

// This file has a type matcher: given a collection of types in an interfaceMap,
// and a type to match against, will find the best match (if there is one).

// TODO: rename methods to lowercase.

import (
	"fmt"
	"reflect"
)

type interfaceMap map[typeCode]*interfaceMatchData

type interfaceMatchData struct {
	name     string
	typeCode typeCode
	layer    int
	plist    []*provider
	consumed bool
}

// Add() inserts a type into an interface map.  The layer
// parameter represents the provider sequence number.  When
// looking for a match, a type with a higher layer number
// is considered to be closer in the call chain than a type
// with a lower layer number.
func (m interfaceMap) Add(t typeCode, layer int, fm *provider) {
	if m[t] != nil {
		m[t].plist = append(m[t].plist, fm)
		return
	}
	m[t] = &interfaceMatchData{
		name:     t.Type().String(),
		typeCode: t,
		layer:    layer,
		plist:    []*provider{fm},
	}
}

func aGreaterBInts(a []int, b []int) bool {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] > b[i] {
			return true
		}
		if a[i] < b[i] {
			return false
		}
	}
	return len(a) >= len(b)
}

func (m interfaceMap) bestMatch(match typeCode, purpose string) (typeCode, []*provider, error) {
	d, found := m[match]
	if found {
		d.consumed = true
		return match, d.plist, nil
	}
	if match.Type().Kind() != reflect.Interface {
		return match, nil, fmt.Errorf("has no match for its %s parameter %s", purpose, match)
	}
	// What is the best match?
	// (*) Highest layer number
	// (*) Same package path for source and destination
	// (*) Highest method count
	// (*) Lowest typeCode value
	var best struct {
		tc    typeCode
		imd   *interfaceMatchData
		score []int
	}
	score := func(tc typeCode, imd *interfaceMatchData) []int {
		samePathScore := 0
		if imd.typeCode.Type().PkgPath() == match.Type().PkgPath() {
			samePathScore = 1
		}
		return []int{imd.layer, samePathScore, imd.typeCode.Type().NumMethod(), int(tc)}
	}
	for tc, imd := range m {
		if !imd.typeCode.Type().Implements(match.Type()) {
			continue
		}
		s := score(tc, imd)
		if best.imd == nil || aGreaterBInts(s, best.score) {
			best.tc = tc
			best.imd = imd
			best.score = s
			continue
		}
	}
	if best.imd == nil {
		return match, nil, fmt.Errorf("has no match for its %s parameter %s", purpose, match)
	}
	loose := looseOnly(match, best.imd.plist)
	if len(loose) == 0 {
		return match, nil, fmt.Errorf("has no match for its %s parameter %s (ignoring %s provided by %s)", purpose, match, best.imd.typeCode, best.imd.plist[0])
	}
	return best.tc, loose, nil
}

func looseOnly(match typeCode, plist []*provider) []*provider {
	loose := make([]*provider, 0, len(plist))
	for _, fm := range plist {
		if _, ok := fm.loose[match]; ok {
			loose = append(loose, fm)
		}
	}
	return loose
}
