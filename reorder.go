package nject

import (
	"container/heap"
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

// rememberOriginalOrder sorts the providers by groups before it
// notes their original ordering.
func rememberOriginalOrder(funcs []*provider) ([]*provider, bool) {
	sets := make(map[groupType][]*provider)
	for _, g := range allGroups {
		sets[g] = make([]*provider, 0, len(funcs))
	}
	var someReorder bool
	for _, fm := range funcs {
		sets[fm.group] = append(sets[fm.group], fm)
	}
	funcs = make([]*provider, 0, len(funcs))
	for _, g := range allGroups {
		funcs = append(funcs, sets[g]...)
	}
	for i, fm := range funcs {
		fm.originalPosition = i
		if fm.reorder {
			someReorder = true
		}
	}
	return funcs, someReorder
}

// XXX pre-compute transitive require
// XXX pre-compute MustConsume checker

// XXX reorder and check
// generateCheckers must be called before reorder()
func reorder(funcs []*provider, canRemoveDesired bool) []*provider {
	fmt.Println("XXX begin reorder ----------------------------------------------------------")
	receivedTypes := make(map[reflect.Type]int)
	providedTypes := make(map[reflect.Type]int)

	counter := len(funcs) + 1

	strongPairs := make([][2]int, 0, len(funcs)*6) // required relationships
	weakPairs := make([][2]int, 0, len(funcs)*6)   // desired relationships
	aAfterB := func(pairs *[][2]int, i, j int) {
		if i == -1 || j == -1 {
			return
		}
		fmt.Println("XXX", i, "comes after", j)
		*pairs = append(*pairs, [2]int{i, j})
	}
	provideByNotRequire := make(map[reflect.Type][]*provider)
	for i, fm := range funcs {
		for _, t := range fm.d.provides {
			if !fm.d.hasRequire(t) {
				provideByNotRequire[t] = append(provideByNotRequire[t], i)
			}
		}
	}

	excluded := make([]*provider, 0, len(funcs))
	cannotReorder := make([]int, 0, len(funcs))
	for i, fm := range funcs {
		if !fm.include {
			excluded = append(excluded, fm)
			continue
		}
		fmt.Println("XXX", i, "is", fm)
		if !fm.reorder {
			cannotReorder = append(cannotReorder, i)
		}
		for _, t := range fm.d.requires {
			if num, ok := providedTypes[t]; ok {
				aAfterB(&strongPairs, i, num)
			} else {
				fmt.Println("XXX downtype", counter, t)
				providedTypes[t] = counter
				aAfterB(&strongPairs, i, counter)
				counter++
			}
			for _, j := range provideByNotRequire[t] {
				aAaferB(&weakPairs, i, j)
			}
		}

		// XXX change!
		// if returns but do not receives:
		// 	must be after all receviers that don't produce t
		//	this is WEAK: if a particular recevier is not included, that's
		//	okay as long as some recevier is included
		// if produce and receiver:
		//	must be after any receiver
		//

		for _, t := range fm.d.returns {
			pairs := &strongPairs
			if fm.consumptionOptional {
				pairs = &weakPairs
			}
			if num, ok := receivedTypes[t]; ok {
				aAfterB(pairs, i, num)
			} else {
				fmt.Println("XXX uptype", counter, t)
				receivedTypes[t] = counter
				aAfterB(pairs, i, counter)
				counter++
			}
		}
	}

	type node struct {
		before     map[int]struct{}
		after      map[int]struct{}
		weakBefore map[int]struct{}
		weakAfter  map[int]struct{}
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
		nodes[pair[1]].weakBefore[pair[0]] = struct{}{}
		nodes[pair[0]].weakAfter[pair[1]] = struct{}{}
	}

	// XXX pre-compute transitive require

	unblocked := &IntHeap{}
	heap.Init(unblocked)
	weakBlocked := &IntHeap{}
	heap.Init(weakBlocked)
	t := topo{
		funcs:          funcs,
		nodes:          nodes,
		cannotReorder:  cannotReorder,
		unblocked:      unblocked,
		weakBlocked:    weakBlocked,
		errors:         make([]error, len(funcs)),
		done:           make([]bool, len(funcs)),
		reorderedFuncs: make([]*provider, 0, len(funcs)),
	}
	err := t.run()
	fmt.Println("XXX final order ...", err)
	for i, fm := range reorderedFuncs {
		fmt.Println("XXX", i, fm, fm.cannotInclude)
	}
	return reorderedFuncs, err
}

// topo is the working data for a toplogical sort
type topo struct {
	funcs            []*provider
	nodes            []node
	cannotReorder    []int
	unblocked        *IntHeap // no weak or strong blocks
	weakBlocked      *IntHeap // only weak blocks
	done             []bool   // TODO: use https://pkg.go.dev/github.com/boljen/go-bitmap#Bitmap instead
	errors           []error
	reorderedFuncs   []*provider
	canRemoveDesired bool
}

func (t *topo) overwrite(v *topo) {
	t.nodes = v.nodes
	t.cannotReorder = v.cannotReorder
	t.unblocked = v.unblocked
	t.weakBlocked = v.weakBlocked
	t.done = v.done
	t.errors = v.errors
	t.reorderedFuncs = v.reorderedFuncs
}

func (t *topo) copy() *topo {
	nodes := make([]node, len(t.nodes))
	copy(nodes, t.nodes)
	for i, n := range nodes {
		nodes[i] = node{
			before:     copySet(n.before),
			after:      copySet(n.after),
			weakBefore: copySet(n.before),
			weakAfter:  copySet(n.after),
		}
	}
	cannotReorder := make([]int, len(t.cannotReorder))
	copy(cannotReorder, t.cannotReorder)
	unblocked := make([]IntHeap, len(*t.unblocked))
	copy(unblocked, *t.unblocked)
	weakBlocked := make([]IntHeap, len(*t.weakBlocked))
	copy(weakBlocked, *t.weakBlocked)
	errors := make([]error, len(t.errors))
	copy(errors, t.errros)
	reorderedFuncs := make([]*provider, len(t.reorderedFuncs), len(t.funcs))
	copy(reorderedFuncs, t.reorderedFuncs)
	done := make([]bool, len(t.funcs))
	copy(done, t.done)
	return &topo{
		funcs:            t.funcs,
		nodes:            nodes,
		cannotReorder:    cannotReorder,
		unblocked:        &underlying,
		weakBlocked:      &weakBlocked,
		done:             done,
		errors:           errors,
		reorderedFuncs:   reorderedFuncs,
		canRemoveDesired: t.canRemoveDesired,
	}
}

func (t *topo) release(n, i int) {
	if n >= len(t.funcs) {
		// types only have strong relationships
		fmt.Println("XXX released", n)
		heap.Push(t.unblocked, n)
	} else {
		delete(t.nodes[n].after, i)
		delete(t.nodes[n].weakAfter, i)
		if len(t.nodes[n].after) == 0 {
			fmt.Println("XXX released", n)
			if len(t.nodes[n].weakAfter) == 0 {
				heap.Push(t.unblocked, n)
			} else {
				heap.Push(t.weakBlocked, n)
			}
		} else {
			fmt.Println("XXX cannot release", n, t.nodes[n].after)
		}
	}
}

func (t *topo) run() error {
	for {
		if t.unblocked.Len() > 0 {
			i := heap.Pop(t.unblocked).(int)
			processOne(i, true)
		} else if t.weakBlocked.Len() > 0 {
			i := heap.Pop(t.weakBlocked).(int)
			processOne(i, true)
		} else if len(t.cannotReorder) > 0 {
			i := t.cannotReorder[0]
			t.cannotReorder = t.cannotReorder[1:]
			released := len(t.nodes[i].after) == 0
			if !released {
				fm := funcs[i]
				t.errors[i] = fmt.Errorf("XXX unmet dependency")
				if fm.required {
					return fmt.Errorf("required, but cannot")
				}
				t.processOne(i, false)
			} else {
				t.processOne(i, true)
			}
		} else {
			fmt.Println("XXX all done")
			break
		}
	}
	// XXX check for failed MustConsume
	// XXX copy errors to funcs
	// XXX some re-orderable funcs may not have been copied over.  Do so.
	return nil
}

func (t *topo) releaseNode(i int) {
	for n := range t.nodes[i].before {
		t.release(n, i)
	}
}

func (t *topo) processOne(i int, release bool) {
	fmt.Println("XXX popped", i, released)
	if release {
		if t.done[i] {
			return
		}
		t.done[i] = true
	}
	if i > len(t.funcs) {
		if release {
			t.releaseNode(i)
		}
		return
	}
	if !release {
		fmt.Println("XXX exclude", fm)
		return
	}
	fm := t.funcs[i]

	if fm.mustInclude && !fm.required {
		alternate := t.copy()
		t.releaseProvider(i, fm)
		err := t.run()
		if errors.Is(err, MustConsumeErr) {
			err := alternate.run()
			if err == nil {
				t.overwrite(alternate)
			}
		}
	} else {
		t.releaseProvider(i, fm)
	}
}

func (t *topo) releaseProvider(i int, fm *provider) {
	fmt.Println("XXX include", fm)
	t.reorderedFuncs = append(t.reorderedFuncs, fm)
	for _, t := range fm.d.provides {
		if num, ok := providedTypes[t]; ok {
			fmt.Println("XXX release down", t)
			t.release(num, i)
		}
	}
	for _, t := range fm.d.recevies {
		if num, ok := receivedTypes[t]; ok {
			fmt.Println("XXX release up", t)
			t.release(num, i)
		}
	}
}
