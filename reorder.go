package nject

import (
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
		fm.reorder = true
		fm.nonFinal = true
	})
}

// Reorder collection to allow providers marked Reorder() to float to their
// necessary spots.  This works by building a dependency graph based upon the
// inputs and outputs of each function.
func (c Collection) reorder() {
	var foundReorder bool
	for _, fm := range c.contents {
		if fm.reorder {
			foundReorder = true
			break
		}
	}
	if !foundReorder {
		return nil
	}

	type firstLast struct {
		first map[reflect.Type]int
		last  map[reflect.Type]int
	}
	type inOut struct {
		in  firstLast
		out firstLast
	}
	type upDown struct {
		up   inOut
		down inOut
	}
	var flows upDown

	noteFirsts := func(i, m *map[reflect.Type]int, typ reflect.Type, direction func(int, int) bool) {
		if m == nil {
			*m = make(map[reflec.Type]int)
		}
		if first, ok := (*m)[typ]; ok {
			if direction(first, i) {
				(*m)[typ] = i
			}
		} else {
			(*m)[typ] = i
		}
	}
	noteInOut := func(i, firstLast *firstLast, types []reflec.Type, direction func(int, int) bool) {
		for _, typ := range types {
			noteFirsts(i, &firstLast.first, typ, direction)
			noteFirsts(i, &firstLast.last, typ, func(i, j) bool { return direction(j, i) })
		}
	}
	note := func(i int, inOut *inOut, flow func() ([]reflect.Type, []reflect.Type), direction func(int, int) bool) {
		in, out := flow()
		noteInOut(i, &inOut.in, in, direction)
		noteInOut(i, &inOut.out, out, direction)
	}
	for i, fm := range c.contents {
		if !fm.reorder {
			note(i, &flows.down, fm.DownFlows, func(i, j int) bool { return i > j })
			note(i, &flows.up, fm.UpFlows, func(i, j int) bool { return i < j })
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
	for i, fm := range c.contents {
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

	desireAAfterB := func(i, j int) {
		if i == -1 || j == -1 {
			return
		}
		potentialDesires = append(potentialDesires, [2]int{i, j})
	}
	lookup := func(inOut inOut, typ reflect.Type) (firstProduce, lastProduce, firstUse, lastUse int) {
		if firstProduce, ok := out.first[typ]; !ok {
			firstProduce = -1
		}
		if lastProduce, ok := out.last[typ]; !ok {
			lastProduce = -1
		}
		if firstUse, ok := in.first[typ]; !ok {
			firstUse = -1
		}
		if lastUse, ok := in.last[typ]; !ok {
			lastUse = -1
		}
	}
	isOrdered = func(i, j int) bool { return i < j }
	floatDown := func(i int, inOut inOut, noSwap func(int, int) (int, int), inOut func() ([]reflect.Type, []reflect.Type)) {
		inputs, outputs := inOut
		hasInput := has(inputs)
		hasOutput := has(outputs)
		firstProduce, lastProduce, firstUse, lastUse := lookup(inOut, typ)
		for _, typ := range inputs {
			if hasOutput(typ) {
				// We both take and produce the same value
				requireAAfterB(swap(i, firstProduce))
				requireAAfterB(swap(lastUse, i))
				desireAAfterB(swap(i, lastProduce))
				desireAAfterB(swap(firstUse, i))
			} else {
				requireAAfterB(swap(i, firstProduce))
				desireAAfterB(swap(i, lastProduce))
			}
		}
		for _, typ := range outputs {
			if hasInput(typ) {
				// handled above
				continue
			}
			requireAAfterB(swap(lastUse, i))
			desireAAfterB(swap(firstUse, i))
			if isOrdered(swap(lastProduce, firstUse)) {
				desireAAfterB(swap(i, lastProduce))
			}
		}
	}
	for i, fm := range c.contents {
		if !fm.reorder {
			continue
		}
		floatDown(i, flows.down, func(i, j int) { return i, j }, fm.DownFlows)
		floatUp(i, flows.up, func(i, j int) { return j, i }, fm.UpFlows)
	}

	// Establish all requirements before putting in the desires
	var desires := make([][2]int, 0, len(potentialDesires))
	for _, potential := range potentialDesires {
		i, j = potential[0], potential[1]
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
		return
	}
	tmp := make([]*provider, len(c.contents))
	copy(tmp, c.contents)
	c.contents = make([]*provider, 0, len(c.contents))
	for _, i := range newOrder {
		c.contents = append(c.contents, tmp[i])
	}
}
