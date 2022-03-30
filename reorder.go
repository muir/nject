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
// Providers annotated as Reorder are also marked as NonFinal.  Marking
// Reorder providers as NotCacheable is usually a good idea.
//
// When reordering, only exact type matches are considered.  Reorder does
// not play well with Loose().
//
// Note: reordering may happen too late for UpFlows(), DownFlows(), and
// GenerateFromInjectionChain() to correctly capture the final shape.
//
// Reorder should be considered experimental in the sense that the rules
// for placement of such providers are likely to be adjusted as feedback
// arrives.
func Reorder(fn interface{}) Provider {
	return newThing(fn).modify(func(fm *provider) {
		fmt.Println("XXX set reorder")
		fm.reorder = true
		fm.nonFinal = true
	})
}

// Reorder collection to allow providers marked Reorder() to float to their
// necessary spots.  This works by building a dependency graph based upon the
// inputs and outputs of each function.
func (c Collection) reorder() {
	fmt.Println("XXX start reorder")
	var foundReorder bool
	for _, fm := range c.contents {
		if fm.reorder {
			foundReorder = true
			break
		}
	}
	if !foundReorder {
		fmt.Println("XXX no reorder")
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
	for i, fm := range c.contents {
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
		c.contents[i].after[j] = struct{}{}
		c.contents[j].before[i] = struct{}{}
	}
	for _, fm := range c.contents {
		fm.before = make(map[int]struct{})
		fm.after = make(map[int]struct{})
		fm.reordered = false
	}
	prior := -1
	for i, fm := range c.contents {
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
	for i, fm := range c.contents {
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
		if _, ok := c.contents[i].after[j]; ok {
			continue
		}
		desires = append(desires, [2]int{i, j})
		requireAAfterB(i, j)
	}

	free := make([]int, 0, len(c.contents))
	for i, fm := range c.contents {
		if len(fm.after) == 0 {
			free = append(free, i)
		}
	}

	newOrder := make([]int, 0, len(c.contents))
	for len(newOrder) < len(c.contents) && (len(free) > 0 || len(desires) > 0) {
		for len(free) > 0 {
			i := free[0]
			free = free[1:]
			fm := c.contents[i]
			fm.reordered = true
			before := make([]int, 0, len(fm.before))
			for b := range fm.before {
				before = append(before, b)
			}
			sort.Sort(sort.IntSlice(before)) // determanistic behavior
			for _, b := range before {
				delete(c.contents[b].after, i)
				if len(c.contents[b].after) == 0 {
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
			if _, ok := c.contents[i].after[j]; ok {
				delete(c.contents[i].after, j)
				delete(c.contents[j].before, i)
				if len(c.contents[i].after) == 0 {
					free = append(free, i)
					break
				}
			}
		}
	}
	if len(newOrder) < len(c.contents) {
		// found a dependency loop
		fmt.Println("XXX found a dependency loop, giving up on reorder")
		return
	}
	tmp := make([]*provider, len(c.contents))
	copy(tmp, c.contents)
	c.contents = make([]*provider, 0, len(c.contents))
	for _, i := range newOrder {
		fmt.Println("XXX, new order", tmp[i])
		c.contents = append(c.contents, tmp[i])
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
