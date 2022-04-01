package nject

import (
	"fmt" //XXX
	"reflect"
	"sort"
)

// Reorder annotates a provider to say that its position in the injection
// chain is not fixed.  Such a provider will be placed after it's inputs
// are available and before it's outputs are consumed.
//
// If there are multiple pass-through providers (that is to say, ones that
// both consume and provide the same type) that pass through the same type,
// then the ordering among these re-orderable providers is not defined but
// will be implemented deterministically.
//
// When reordering, only exact type matches are considered.  Reorder does
// not play well with Loose().
//
// Note: reordering will happen too late for UpFlows(), DownFlows(), and
// GenerateFromInjectionChain() to correctly capture the final shape.
//
// Reorder should be considered experimental in the sense that the rules
// for placement of such providers are likely to be adjusted as feedback
// arrives.  This initial version requires that Reorder()ed producers are
// before the first consumer of that type.  This strict placement may be
// relaxed in a future release of nject.
func Reorder(fn interface{}) Provider {
	return newThing(fn).modify(func(fm *provider) {
		fm.reorder = true
	})
}

// Reorder re-arranges an array of funcs with the functions that are
// marked Reorder potentially moving to no positions within the array.
//
// This is accomplished by looking at what's provided and consumed and
// make sure that the consumers are after the providers.
//
// The challenge comes when there is more than one provider or when the
// same type is both provided and consumed.
//
// Rules:
//
// Functions that are not marked Reorder must be in their original
// order with respect to each other.
//
// For functions are marked Reorder:
//
//	If function A provides type T (and does not consume type T)
//	then function A must be before any consumers of type T.
//
//	If function B provides and consumes type T, then it must be
//	after the first provider of type T but before the first consumer of T.
//	Function B does not count as consumer of type T for other Reorder
//	functions.
//
//	If function C consumes type T, then it must be after the
//	last producer of type T.
//
// Functions marked Reorder are addedd to the provide/consume graph in
// the order that they're processed.
//
// If a new order isn't possible, that is a fatal error

func rememberOriginalOrder(funcs []*provider) bool {
	var someReorder bool
	for i, fm := range funcs {
		fm.originalPosition = i
		if fm.reorder {
			someReorder = true
		}
	}
	return someReorder
}

func restoreOriginalOrder(funcs []*provider) {
	panic()
}

func reorder(funcs []*provider) error {
	var foundReorder bool
	for _, fm := range funcs {
		if fm.cannotInclude != nil {
			continue
		}
		if fm.reorder {
			foundReorder = true
			break
		}
	}
	if !foundReorder {
		return
	}

	type firstLast struct {
		First map[reflect.Type]int
		Last  map[reflect.Type]int
	}
	type inOut struct {
		In  firstLast
		Out firstLast
	}
	type upDown struct {
		Up   inOut
		Down inOut
	}
	var flows upDown

	noteFirsts := func(i int, m *map[reflect.Type]int, typ reflect.Type, direction func(int, int) bool) {
		if *m == nil {
			*m = make(map[reflect.Type]int)
		}
		if first, ok := (*m)[typ]; ok {
			if direction(first, i) {
				(*m)[typ] = i
			}
		} else {
			(*m)[typ] = i
		}
	}
	noteInOut := func(i int, firstLast *firstLast, types []reflect.Type, direction func(int, int) bool) {
		for _, typ := range types {
			noteFirsts(i, &firstLast.First, typ, direction)
			noteFirsts(i, &firstLast.Last, typ, func(i, j int) bool { return direction(j, i) })
		}
	}
	note := func(i int, inOut *inOut, flow func() ([]reflect.Type, []reflect.Type), direction func(int, int) bool) {
		in, out := flow()
		noteInOut(i, &inOut.In, in, direction)
		noteInOut(i, &inOut.Out, out, direction)
	}
	for i, fm := range funcs {
		if !fm.reorder {
			note(i, &flows.Down, fm.DownFlows, func(i, j int) bool { return i > j })
			note(i, &flows.Up, fm.UpFlows, func(i, j int) bool { return i < j })
		}
	}

	// We will eventually pull dependencies out of the graph based upon what
	// doesn't need to be before anything -- that is to say, we'll pull item 0
	// first in the normal situation.  That's the "outermost"
	requireAAfterB := func(i, j int) {
		if i == -1 || j == -1 {
			return
		}
		funcs[i].after[j] = struct{}{}
		funcs[j].before[i] = struct{}{}
	}
	for _, fm := range funcs {
		fm.before = make(map[int]struct{})
		fm.after = make(map[int]struct{})
		fm.reordered = false
	}
	prior := -1
	for i, fm := range funcs {
		if !fm.reorder {
			requireAAfterB(i, prior)
			prior = i
		}
	}

	var potentialDesires [][2]int
	desireAAfterB := func(i, j int) {
		if i == -1 || j == -1 {
			return
		}
		potentialDesires = append(potentialDesires, [2]int{i, j})
	}
	lookup := func(inOut inOut, typ reflect.Type) (firstProduce, lastProduce, firstUse, lastUse int) {
		var ok bool
		if firstProduce, ok = inOut.Out.First[typ]; !ok {
			firstProduce = -1
		}
		if lastProduce, ok = inOut.Out.Last[typ]; !ok {
			lastProduce = -1
		}
		if firstUse, ok = inOut.In.First[typ]; !ok {
			firstUse = -1
		}
		if lastUse, ok = inOut.In.Last[typ]; !ok {
			lastUse = -1
		}
		return
	}
	isOrdered := func(i, j int) bool { return i < j }
	floatDown := func(i int, inOut inOut, noSwap func(int, int) (int, int), inputsOutputs func() ([]reflect.Type, []reflect.Type)) {
		inputs, outputs := inputsOutputs()
		hasInput := has(inputs)
		hasOutput := has(outputs)
		for _, typ := range inputs {
			firstProduce, lastProduce, firstUse, lastUse := lookup(inOut, typ)
			if hasOutput(typ) {
				// We both take and produce the same value
				requireAAfterB(noSwap(i, firstProduce))
				requireAAfterB(noSwap(lastUse, i))
				desireAAfterB(noSwap(i, lastProduce))
				desireAAfterB(noSwap(firstUse, i))
			} else {
				requireAAfterB(noSwap(i, firstProduce))
				desireAAfterB(noSwap(i, lastProduce))
			}
		}
		for _, typ := range outputs {
			if hasInput(typ) {
				// handled above
				continue
			}
			_, lastProduce, firstUse, lastUse := lookup(inOut, typ)
			requireAAfterB(noSwap(lastUse, i))
			desireAAfterB(noSwap(firstUse, i))
			if isOrdered(noSwap(lastProduce, firstUse)) {
				desireAAfterB(noSwap(i, lastProduce))
			}
		}
	}
	for i, fm := range funcs {
		if !fm.reorder {
			continue
		}
		floatDown(i, flows.Down, func(i, j int) (int, int) { return i, j }, fm.DownFlows)
		floatDown(i, flows.Up, func(i, j int) (int, int) { return j, i }, fm.UpFlows)
	}

	// Establish all requirements before putting in the desires
	desires := make([][2]int, 0, len(potentialDesires))
	for _, potential := range potentialDesires {
		i, j := potential[0], potential[1]
		if _, ok := funcs[i].after[j]; ok {
			continue
		}
		desires = append(desires, [2]int{i, j})
		requireAAfterB(i, j)
	}

	free := make([]int, 0, len(funcs))
	for i, fm := range funcs {
		if len(fm.after) == 0 {
			free = append(free, i)
		}
	}

	newOrder := make([]int, 0, len(funcs))
	for len(newOrder) < len(funcs) && (len(free) > 0 || len(desires) > 0) {
		for len(free) > 0 {
			i := free[0]
			free = free[1:]
			fm := funcs[i]
			fm.reordered = true
			before := make([]int, 0, len(fm.before))
			for b := range fm.before {
				before = append(before, b)
			}
			sort.Sort(sort.IntSlice(before)) // determanistic behavior
			for _, b := range before {
				delete(funcs[b].after, i)
				if len(funcs[b].after) == 0 {
					free = append(free, b)
				}
			}
		}
		if len(free) == 0 {
			break
		}

		for len(desires) > 0 && len(free) == 0 {
			i, j := desires[0][0], desires[0][1]
			desires = desires[1:]
			if _, ok := funcs[i].after[j]; ok {
				delete(funcs[i].after, j)
				delete(funcs[j].before, i)
				if len(funcs[i].after) == 0 {
					free = append(free, i)
					break
				}
			}
		}
	}
	if len(newOrder) < len(funcs) {
		// found a dependency loop
		fmt.Println("XXX found a dependency loop, giving up on reorder")
		return
	}
	tmp := make([]*provider, len(funcs))
	copy(tmp, funcs)
	funcs = make([]*provider, 0, len(funcs))
	for _, i := range newOrder {
		fmt.Println("XXX, new order", tmp[i])
		funcs = append(funcs, tmp[i])
	}
}

func has(types []reflect.Type) func(reflect.Type) bool {
	m := make(map[reflect.Type]struct{})
	for _, typ := range types {
		m[typ] = struct{}{}
	}
	return func(typ reflect.Type) bool {
		_, ok := m[typ]
		return ok
	}
}
