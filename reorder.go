package nject

import (
	"container/heap"
	"errors"
	"fmt" //XXX
	"reflect"
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
//	a pseudo node that represents that T has been recevied as
//	as a return value in the up-chain.
//

// XXX pre-compute transitive require
// XXX pre-compute MustConsume checker

// XXX reorder and check
// generateCheckers must be called before reorder()
func reorder(funcs []*provider, initF *provider) ([]*provider, error) {
	fmt.Println("XXX begin reorder ----------------------------------------------------------")
	var someReorder bool
	for i, fm := range funcs {
		fmt.Println("XXX START", i, fm, fm.cannotInclude, fm.include)
		if fm.reorder {
			someReorder = true
		}
	}
	if !someReorder {
		return funcs, nil
	}

	downAvailable := make(interfaceMap)
	upAvailable := make(interfaceMap)

	provideByNotRequire := make(map[typeCode][]int)
	if initF != nil {
		for _, t := range noNoType(fm.flows[outputParams]) {
			downAvailable.Add(t, 0, initF)
			provideByNotRequire[t] = append(provideByNotRequire[t], -1)
		}
	}

	lastStatic := -1
	for i, fm := range funcs {
		for j := 0; j < lastFlowType; j++ {
			fm.hasFlow[j] = has(noNoType(fm.flow[j]))
		}
		for _, t := range noNoType(fm.flows[outputParams]) {
			downAvailable.Add(t, i, fm)
		}
		for _, t := range noNoType(fm.flows[returnParams]) {
			upAvailable.Add(t, i, fm)
		}
		if fm.class == staticClass && !fm.reorder {
			lastStatic = i
		}
	}

	receivedTypes := make(map[typeCode]int)
	providedTypes := make(map[typeCode]int)

	counter := len(funcs) + 1
	receviedNotReturned := make(map[typeCode][]int)
	for i, fm := range funcs {
		for _, t := range noNoType(fm.flows[outputParams]) {
			if !fm.d.hasFlow[inputParams](t) {
				provideByNotRequire[t] = append(provideByNotRequire[t], i)
			}
		}
		for _, t := range noNoType(fm.flows[receviedParams]) {
			if !fm.d.hasFlow[returnedParams](t) {
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
		fmt.Println("XXX", i, "comes after", j, map[bool]string{
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
	excluded := make([]*provider, 0, len(funcs))
	cannotReorder := make([]int, 0, len(funcs))
	for i, fm := range funcs {
		if fm.reorder && fm.class != staticClass {
			// All reorder functions must be after the end of the
			// static set
			aAfterB(true, i, lastStatic)
		}
		fmt.Println("XXX", i, "is", fm)
		if !fm.reorder {
			// providers that cannot be reordered will be forced out
			// one after another
			cannotReorder = append(cannotReorder, i)
		}
		for _, tRaw := range fm.d.requires {
			t, _, err := availableDown.bestMatch(tRaw, "downflow")
			if err != nil {
				return nil, err
			}
			// If you take a T as input, you MUST be after some provider that outputs a T
			if num, ok := providedTypes[t]; ok {
				aAfterB(true, i, num)
			} else {
				fmt.Println("XXX downtype", counter, t)
				providedTypes[t] = counter
				aAfterB(true, i, counter)
				counter++
			}
			// If you take a T, then you SHOULD be after all providers of T that don't
			// themselves take a T.
			for _, j := range provideByNotRequire[t] {
				aAfterB(false, i, j)
			}
		}

		for _, tRaw := range fm.d.returns {
			t, _, err := availableUp.bestMatch(tRaw, "downflow")
			if err != nil {
				return nil, err
			}
			// if you return a T, you're not marked consumptionOptional, then you
			// MUST be be after a provider that recevies a T as a returned value
			if num, ok := receivedTypes[t]; ok {
				aAfterB(!fm.consumptionOptional, i, num)
			} else {
				fmt.Println("XXX uptype", counter, t)
				receivedTypes[t] = counter
				aAfterB(!fm.consumptionOptional, i, counter)
				counter++
			}
			// if you return a T, then you SHOULD be be after all providers that
			// recevie a T as a returned value
			for _, j := range receviedNotReturned[t] {
				aAfterB(false, i, j)
			}
		}
	}

	nodes := make([]node, counter)
	for i, fm := range funcs {
		if fm.include {
			nodes[i] = node{
				before:     make(map[int]struct{}),
				after:      make(map[int]struct{}),
				weakBefore: make(map[int]struct{}),
				weakAfter:  make(map[int]struct{}),
			}
		}
	}
	for i := len(funcs); i < counter; i++ {
		nodes[i] = node{
			before: make(map[int]struct{}),
			after:  make(map[int]struct{}),
		}
	}
	for _, pair := range strongPairs {
		nodes[pair[1]].before[pair[0]] = struct{}{}
		nodes[pair[0]].after[pair[1]] = struct{}{}
	}
	for _, pair := range weakPairs {
		fmt.Println("XXX", pair, len(funcs))
		nodes[pair[1]].weakBefore[pair[0]] = struct{}{}
		nodes[pair[0]].weakAfter[pair[1]] = struct{}{}
	}

	unblocked := &IntHeap{}
	heap.Init(unblocked)
	weakBlocked := &IntHeap{}
	heap.Init(weakBlocked)

	if initF != nil {
		for _, t := range noNoType(initF.flows[outputParams]) {
			if num, ok := providedTypes[t]; ok {
				fmt.Println("XXX release down for InitF", t)
				heap.Push(unblocked, num)
				x.release(num, i)
			}
		}
	}

	x := topo{
		funcs:          funcs,
		nodes:          nodes,
		cannotReorder:  cannotReorder,
		unblocked:      unblocked,
		weakBlocked:    weakBlocked,
		done:           make([]bool, len(funcs)),
		reorderedFuncs: make([]*provider, 0, len(funcs)),
		receivedTypes:  receivedTypes,
		providedTypes:  providedTypes,
	}
	err := x.run()
	fmt.Println("XXX final order ...", err)
	for i, fm := range x.reorderedFuncs {
		fmt.Println("XXX", i, fm, fm.cannotInclude)
	}
	return x.reorderedFuncs, err
}

type node struct {
	before     map[int]struct{}
	after      map[int]struct{}
	weakBefore map[int]struct{}
	weakAfter  map[int]struct{}
}

// XXX all non-static must be after all static

// topo is the working data for a toplogical sort
type topo struct {
	funcs          []*provider
	nodes          []node
	cannotReorder  []int
	unblocked      *IntHeap // no weak or strong blocks
	weakBlocked    *IntHeap // only weak blocks
	done           []bool   // TODO: use https://pkg.go.dev/github.com/boljen/go-bitmap#Bitmap instead
	reorderedFuncs []*provider
	check          func([]*provider) error
	receivedTypes  map[reflect.Type]int
	providedTypes  map[reflect.Type]int
}

func (x *topo) overwrite(v *topo) {
	x.nodes = v.nodes
	x.cannotReorder = v.cannotReorder
	x.unblocked = v.unblocked
	x.weakBlocked = v.weakBlocked
	x.done = v.done
	x.reorderedFuncs = v.reorderedFuncs
}

func (x *topo) copy() *topo {
	nodes := make([]node, len(x.nodes))
	copy(nodes, x.nodes)
	for i, n := range nodes {
		nodes[i] = node{
			before:     copySet(n.before),
			after:      copySet(n.after),
			weakBefore: copySet(n.before),
			weakAfter:  copySet(n.after),
		}
	}
	cannotReorder := make([]int, len(x.cannotReorder))
	copy(cannotReorder, x.cannotReorder)

	unblocked := make(IntHeap, len(*x.unblocked))
	copy(unblocked, *x.unblocked)

	weakBlocked := make(IntHeap, len(*x.weakBlocked))
	copy(weakBlocked, *x.weakBlocked)

	reorderedFuncs := make([]*provider, len(x.reorderedFuncs), len(x.funcs))
	copy(reorderedFuncs, x.reorderedFuncs)

	done := make([]bool, len(x.funcs))
	copy(done, x.done)

	return &topo{
		funcs:          x.funcs,
		nodes:          nodes,
		cannotReorder:  cannotReorder,
		unblocked:      &unblocked,
		weakBlocked:    &weakBlocked,
		done:           done,
		reorderedFuncs: reorderedFuncs,
		check:          x.check,
		receivedTypes:  x.receivedTypes,
		providedTypes:  x.providedTypes,
	}
}

func copySet(s map[int]struct{}) map[int]struct{} {
	n := make(map[int]struct{})
	for k := range s {
		n[k] = struct{}{}
	}
	return n
}

func (x *topo) release(n, i int) {
	if n >= len(x.funcs) {
		// types only have strong relationships
		fmt.Println("XXX released", n)
		heap.Push(x.unblocked, n)
	} else {
		delete(x.nodes[n].after, i)
		delete(x.nodes[n].weakAfter, i)
		if len(x.nodes[n].after) == 0 {
			fmt.Println("XXX released", n)
			if len(x.nodes[n].weakAfter) == 0 {
				heap.Push(x.unblocked, n)
			} else {
				heap.Push(x.weakBlocked, n)
			}
		} else {
			fmt.Println("XXX cannot release", n, x.nodes[n].after)
		}
	}
}

func (x *topo) run() error {
	for {
		if x.unblocked.Len() > 0 {
			i := heap.Pop(x.unblocked).(int)
			x.processOne(i, true)
		} else if x.weakBlocked.Len() > 0 {
			i := heap.Pop(x.weakBlocked).(int)
			x.processOne(i, true)
		} else if len(x.cannotReorder) > 0 {
			i := x.cannotReorder[0]
			x.cannotReorder = x.cannotReorder[1:]
			released := len(x.nodes[i].after) == 0
			if !released {
				fm := x.funcs[i]
				fm.cannotInclude = fmt.Errorf("XXX unmet dependency")
				if fm.required {
					return fmt.Errorf("required, but cannot")
				}
				x.processOne(i, false)
			} else {
				x.processOne(i, true)
			}
		} else {
			fmt.Println("XXX all done")
			break
		}
	}

	for i, fm := range x.funcs {
		if !x.done[i] {
			fm.cannotInclude = fm.errorf("dependencies not met, excluded")
			x.reorderedFuncs = append(x.reorderedFuncs, fm)
		}
	}
	return x.check(x.reorderedFuncs)
}

func (x *topo) releaseNode(i int) {
	for n := range x.nodes[i].before {
		x.release(n, i)
	}
}

func (x *topo) processOne(i int, release bool) {
	fmt.Println("XXX popped", i, release)
	if release {
		if x.done[i] {
			return
		}
		x.done[i] = true
	}
	if i > len(x.funcs) {
		if release {
			x.releaseNode(i)
		}
		return
	}
	fm := x.funcs[i]
	if !release {
		fmt.Println("XXX exclude", fm)
		return
	}

	/* XXX
	if fm.mustConsume && !fm.required {
		alternate := x.copy()
		errorsCopy := make([]error, len(x.funcs))
		for i, fm := range x.funcs {
			errorsCopy[i] = fm.cannotInclude
		}
		x.releaseProvider(i, fm)
		err := x.run()
		if errors.Is(err, MustConsumeError{}) {
			for i, fm := range x.funcs {
				fm.cannotInclude = errorsCopy[i]
			}
			err := alternate.run()
			if err == nil {
				x.overwrite(alternate)
			}
		}
	} else {
	*/
	x.releaseProvider(i, fm)
	// XXX }
}

func (x *topo) releaseProvider(i int, fm *provider) {
	fmt.Println("XXX include", fm)
	x.reorderedFuncs = append(x.reorderedFuncs, fm)
	for _, t := range noNoType(fm.flows[outputParams]) {
		if num, ok := x.providedTypes[t]; ok {
			fmt.Println("XXX release down", t)
			x.release(num, i)
		}
	}
	for _, t := range noNoType(fm.flows[receviedParams]) {
		if num, ok := x.receivedTypes[t]; ok {
			fmt.Println("XXX release up", t)
			x.release(num, i)
		}
	}
}
