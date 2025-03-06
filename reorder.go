package nject

import (
	"container/heap"
	"fmt"
)

// Reorder annotates a provider to say that its position in the injection
// chain is not fixed.  Such a provider will be placed after it's inputs
// are available and before it's outputs are consumed.
//
// If there are multiple pass-through providers (that is to say, ones that
// both consume and provide the same type) that pass through the same type,
// then the ordering among these re-orderable providers will be in their
// original order with respect to each other.
//
// When reordering, only exact type matches are considered.  Reorder does
// not play well with Loose().
//
// Functions marked reorder are currently inelligible for the STATIC set.
//
// Note: reordering will happen too late for UpFlows(), DownFlows(), and
// GenerateFromInjectionChain() to correctly capture the final shape.
//
// Reorder should be considered experimental in the sense that the rules
// for placement of such providers are likely to be adjusted as feedback
// arrives.
func Reorder(fn any) Provider {
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
//
// Rules:
//
//	A provider can be strictly after another provider.  This is
//	what's done to handle providers that are not marked Reorder.
//
//	A provider that consumes a type T on the downchain is after
//	a pseudo node that represents that T has been provided.
//
//	A provider that returns a type T on the up-chain, is after
//	a pseudo node that represents that T has been received as
//	as a return value in the up-chain.
//

// generateCheckers must be called before reorder()
func reorder(funcs []*provider, initF *provider) ([]*provider, error) {
	debugln("begin reorder ----------------------------------------------------------")
	var someReorder bool
	for i, fm := range funcs {
		debugln("\tSTART", i, fm, fm.cannotInclude, fm.include)
		if fm.reorder {
			someReorder = true
		}
	}
	if !someReorder {
		return funcs, nil
	}

	availableDown := make(interfaceMap)
	availableUp := make(interfaceMap)

	provideByNotRequire := make(map[typeCode][]int)
	if initF != nil {
		for _, t := range noNoType(initF.flows[outputParams]) {
			availableDown.Add(t, 0, initF)
			provideByNotRequire[t] = append(provideByNotRequire[t], -1)
		}
	}

	lastStatic := -1
	for i, fm := range funcs {
		for j := flowType(0); j < lastFlowType; j++ {
			fm.d.hasFlow[j] = has(fm.flows[j])
		}
		for _, t := range noNoType(fm.flows[outputParams]) {
			availableDown.Add(t, i, fm)
		}
		for _, t := range noNoType(fm.flows[returnParams]) {
			availableUp.Add(t, i, fm)
		}
		if fm.group == staticGroup && !fm.reorder {
			lastStatic = i
		}
	}

	upTypes := make(map[typeCode]int)
	downTypes := make(map[typeCode]int)

	counter := len(funcs) + 1
	receviedNotReturned := make(map[typeCode][]int)
	for i, fm := range funcs {
		for _, t := range noNoType(fm.flows[outputParams]) {
			if !fm.d.hasFlow[inputParams](t) {
				provideByNotRequire[t] = append(provideByNotRequire[t], i)
			}
		}
		for _, t := range noNoType(fm.flows[receivedParams]) {
			if !fm.d.hasFlow[returnParams](t) {
				receviedNotReturned[t] = append(receviedNotReturned[t], i)
			}
		}
	}

	strongPairs := make([][2]int, 0, len(funcs)*6) // required relationships
	weakPairs := make([][2]int, 0, len(funcs)*6)   // desired relationships
	aAfterB := func(strong bool, i, j int) {
		if i == -1 || j == -1 {
			return
		}
		debugln("\t", i, "comes after", j, map[bool]string{
			false: "weak",
			true:  "strong",
		}[strong])
		var pp *[][2]int
		if strong {
			pp = &strongPairs
		} else {
			pp = &weakPairs
		}
		*pp = append(*pp, [2]int{i, j})
	}
	cannotReorder := make([]int, 0, len(funcs))
	lastNoReorder := -1
	for i, fm := range funcs {
		if fm.reorder && fm.group == runGroup {
			// All reorder functions must be after the end of the
			// static set
			aAfterB(true, i, lastStatic)
		}
		debugln("\t", i, "is", fm)
		if !fm.reorder {
			// providers that cannot be reordered will be forced out
			// one after another
			cannotReorder = append(cannotReorder, i)
			aAfterB(true, i, lastNoReorder)
			lastNoReorder = i
		}
		for _, tRaw := range noNoType(fm.flows[inputParams]) {
			t, _, err := availableDown.bestMatch(tRaw, "downflow")
			if err != nil {
				// we'll simply ignore the type since it cannot be provided
				continue
			}
			// If you take a T as input, you MUST be after some provider that outputs a T
			if num, ok := downTypes[t]; ok {
				aAfterB(true, i, num)
			} else {
				debugln("\tdowntype", counter, t)
				downTypes[t] = counter
				aAfterB(true, i, counter)
				counter++
			}
			// If you take a T, then you SHOULD be after all providers of T that don't
			// themselves take a T.
			for _, j := range provideByNotRequire[t] {
				aAfterB(false, i, j)
			}
		}

		for _, tRaw := range noNoType(fm.flows[returnParams]) {
			t, _, err := availableUp.bestMatch(tRaw, "upflow")
			if err != nil {
				// we'll simply ignore the type since it cannot be provided
				continue
			}
			// if you return a T, you're not marked consumptionOptional, then you
			// MUST be after a provider that receives a T as a returned value
			_, consumptionOptional := fm.consumptionOptional[t]
			if num, ok := upTypes[t]; ok {
				aAfterB(!consumptionOptional, i, num)
			} else {
				debugln("\tuptype", counter, t)
				upTypes[t] = counter
				aAfterB(!consumptionOptional, i, counter)
				counter++
			}
			// if you return a T, then you SHOULD be be after all providers that
			// receive a T as a returned value
			for _, j := range receviedNotReturned[t] {
				aAfterB(false, i, j)
			}
		}
	}

	nodes := make([]node, counter)
	for i := range funcs {
		nodes[i] = node{
			before:     make(map[int]struct{}),
			after:      make(map[int]struct{}),
			weakBefore: make(map[int]struct{}),
			weakAfter:  make(map[int]struct{}),
		}
	}
	debugln("\tcounter:", len(funcs), counter)
	for i := len(funcs); i < counter; i++ {
		nodes[i] = node{
			before: make(map[int]struct{}),
			after:  make(map[int]struct{}),
		}
	}
	for _, pair := range strongPairs {
		debugln("\tstrong", pair)
		nodes[pair[1]].before[pair[0]] = struct{}{}
		nodes[pair[0]].after[pair[1]] = struct{}{}
	}
	for _, pair := range weakPairs {
		debugln("\tweak", pair)
		nodes[pair[1]].weakBefore[pair[0]] = struct{}{}
		nodes[pair[0]].weakAfter[pair[1]] = struct{}{}
	}
	for _, pair := range weakPairs {
		if _, ok := nodes[pair[0]].weakBefore[pair[1]]; !ok {
			continue
		}
		debugln("\tremove mutual weak", pair)
		delete(nodes[pair[1]].weakBefore, pair[0])
		delete(nodes[pair[0]].weakBefore, pair[0])
		delete(nodes[pair[0]].weakAfter, pair[1])
		delete(nodes[pair[1]].weakAfter, pair[1])
	}

	unblocked := &intsHeap{}
	heap.Init(unblocked)
	weakBlocked := &intsHeap{}
	heap.Init(weakBlocked)

	if initF != nil {
		for _, t := range noNoType(initF.flows[outputParams]) {
			if num, ok := downTypes[t]; ok {
				debugln("\trelease down for InitF", t)
				push(unblocked, funcs, num)
			}
		}
	}
	x := topo{
		funcs:          funcs,
		nodes:          nodes,
		cannotReorder:  cannotReorder,
		unblocked:      unblocked,
		weakBlocked:    weakBlocked,
		done:           make([]bool, counter),
		reorderedFuncs: make([]*provider, 0, len(funcs)),
		upTypes:        upTypes,
		downTypes:      downTypes,
	}
	x.run()
	debugln("\tfinal order ...")
	for i, fm := range x.reorderedFuncs {
		debugln("\t\t", i, fm)
	}
	debugln("------------------")
	if len(funcs) != len(x.reorderedFuncs) {
		return nil, fmt.Errorf("internal error: count of funcs changed during reorder")
	}
	return x.reorderedFuncs, nil
}

type node struct {
	before     map[int]struct{} // set of nodes that must be released before this node (dependent node is required)
	after      map[int]struct{} // set of nodes that must be released after this node
	weakBefore map[int]struct{} // set of nodes that must be released before this node (dependent node is desired)
	weakAfter  map[int]struct{} // set of nodes that must be released after this node
}

// topo is the working data for a toplogical sort
type topo struct {
	funcs          []*provider
	nodes          []node
	cannotReorder  []int
	unblocked      *intsHeap // no weak or strong blocks
	weakBlocked    *intsHeap // only weak blocks
	done           []bool    // TODO: use https://pkg.go.dev/github.com/boljen/go-bitmap#Bitmap instead
	reorderedFuncs []*provider
	upTypes        map[typeCode]int
	downTypes      map[typeCode]int
}

func (x *topo) release(n, i int) {
	if n >= len(x.funcs) {
		// types only have strong relationships
		debugln("\treleased", n)
		push(x.unblocked, x.funcs, n)
	} else {
		delete(x.nodes[n].after, i)
		delete(x.nodes[n].weakAfter, i)
		if len(x.nodes[n].after) == 0 {
			if len(x.nodes[n].weakAfter) == 0 {
				debugln("\treleased", n)
				push(x.unblocked, x.funcs, n)
			} else {
				debugln("\treleased (weak)", n, x.nodes[n].weakAfter)
				push(x.weakBlocked, x.funcs, n)
			}
		} else {
			debugln("\tcannot release", n, x.nodes[n].after)
		}
	}
}

func (x *topo) releaseNode(i int) {
	for n := range x.nodes[i].weakBefore {
		delete(x.nodes[n].weakAfter, i)
	}
	for n := range x.nodes[i].before {
		x.release(n, i)
	}
}

func (x *topo) run() {
	for {
		if x.unblocked.Len() > 0 {
			i := pop(x.unblocked)
			x.processOne(i, true)
		} else if x.weakBlocked.Len() > 0 {
			i := pop(x.weakBlocked)
			x.processOne(i, true)
		} else if len(x.cannotReorder) > 0 {
			i := x.cannotReorder[0]
			x.cannotReorder = x.cannotReorder[1:]
			released := len(x.nodes[i].after) == 0
			x.processOne(i, released)
		} else {
			debugln("\tall done")
			break
		}
	}

	for i, fm := range x.funcs {
		if !x.done[i] {
			fm.cannotInclude = fm.errorf("dependencies not met, excluded")
			x.reorderedFuncs = append(x.reorderedFuncs, fm)
		}
	}
}

func (x *topo) processOne(i int, release bool) {
	debugln("\tpopped", i, release)
	if x.done[i] {
		return
	}
	x.done[i] = true
	if i > len(x.funcs) {
		if release {
			x.releaseNode(i)
		}
		return
	}
	fm := x.funcs[i]
	x.reorderedFuncs = append(x.reorderedFuncs, fm)
	if !release {
		debugln("\texclude", fm)
		return
	}

	x.releaseNode(i)
	x.releaseProvider(i, fm)
}

func (x *topo) releaseProvider(i int, fm *provider) {
	debugln("\tinclude", fm)
	for _, t := range noNoType(fm.flows[outputParams]) {
		if num, ok := x.downTypes[t]; ok {
			debugln("\trelease down", t)
			x.release(num, i)
		}
	}
	for _, t := range noNoType(fm.flows[receivedParams]) {
		if num, ok := x.upTypes[t]; ok {
			debugln("\trelease up", t)
			x.release(num, i)
		}
	}
}

func has(types []typeCode) func(typeCode) bool {
	m := make(map[typeCode]struct{})
	for _, t := range noNoType(types) {
		m[t] = struct{}{}
	}
	return func(t typeCode) bool {
		_, ok := m[t]
		return ok
	}
}
