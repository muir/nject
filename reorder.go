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
func reorder(funcs []*provider, canRemoveDesired bool) []*provider {
	fmt.Println("XXX begin reorder ----------------------------------------------------------")
	receivedTypes := make(map[reflect.Type]int)
	providedTypes := make(map[reflect.Type]int)

	counter := len(funcs) + 1

	pairs := make([][2]int, 0, len(funcs)*6)
	aAfterB := func(i, j int) {
		if i == -1 || j == -1 {
			return
		}
		fmt.Println("XXX", i, "comes after", j)
		pairs = append(pairs, [2]int{i, j})
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
		in, _ := fm.DownFlows()
		for _, t := range in {
			if num, ok := providedTypes[t]; ok {
				aAfterB(i, num)
			} else {
				fmt.Println("XXX downtype", counter, t)
				providedTypes[t] = counter
				aAfterB(i, counter)
				counter++
			}
		}
		// XXX change!
		// if produce but do not receive:
		// 	must be after all receviers that don't produce t
		//	this is WEAK: if a particular recevier is not included, that's
		//	okay as long as some recevier is included
		// if produce and receiver:
		//	must be after any receiver
		//
		// XXX change!
		// 	if consumptionOptional then only create dependencies where
		//	a recevier of a type exists.  Create a list of such dependencies
		// 	and remove them when stuck.
		_, produce := fm.UpFlows()
		for _, t := range produce {
			if num, ok := receivedTypes[t]; ok {
				aAfterB(i, num)
			} else {
				fmt.Println("XXX uptype", counter, t)
				receivedTypes[t] = counter
				aAfterB(i, counter)
				counter++
			}
		}
	}

	type node struct {
		before map[int]struct{}
		after  map[int]struct{}
	}
	nodes := make([]node, counter)
	for i, fm := range funcs {
		if fm.include {
			nodes[i] = node{
				before: make(map[int]struct{}),
				after:  make(map[int]struct{}),
			}
		}
	}
	for i := len(funcs); i < counter; i++ {
		nodes[i] = node{
			before: make(map[int]struct{}),
			after:  make(map[int]struct{}),
		}
	}
	for _, pair := range pairs {
		nodes[pair[1]].before[pair[0]] = struct{}{}
		nodes[pair[0]].after[pair[1]] = struct{}{}
	}

	// XXX pre-compute transitive require

	var underlying IntHeap
	todo := &underlying
	heap.Init(todo)
	t := topo{
		funcs:          funcs,
		nodes:          nodes,
		cannotReorder:  cannotReorder,
		todo:           todo,
		errors:         make([]error, len(funcs)),
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
	todo             *IntHeap
	errors           []error
	reorderedFuncs   []*provider
	canRemoveDesired bool
}

func (t *topo) overwrite(v *topo) {
	t.nodes = v.nodes
	t.cannotReorder = v.cannotReorder
	t.todo = v.todo
	t.errors = v.errors
	t.reorderedFuncs = v.reorderedFuncs
}

func (t *topo) copy() *topo {
	nodes := make([]node, len(t.nodes))
	copy(nodes, t.nodes)
	for i, n := range nodes {
		nodes[i] = node{
			before: copySet(n.before),
			after:  copySet(n.after),
		}
	}
	cannotReorder := make([]int, len(t.cannotReorder))
	copy(cannotReorder, t.cannotReorder)
	underlying := make([]IntHeap, len(*t.todo))
	copy(underlying, *t.todo)
	errors := make([]error, len(t.errors))
	copy(errors, t.errros)
	reorderedFuncs := make([]*provider, len(t.reorderedFuncs), len(t.funcs))
	copy(reorderedFuncs, t.reorderedFuncs)
	return &topo{
		funcs:            t.funcs,
		nodes:            nodes,
		cannotReorder:    cannotReorder,
		todo:             &underlying,
		errors:           errors,
		reorderedFuncs:   reorderedFuncs,
		canRemoveDesired: t.canRemoveDesired,
	}
}

func (t *topo) release(n, i int) {
	if n >= len(t.funcs) {
		fmt.Println("XXX released", n)
		heap.Push(t.todo, n)
	} else {
		delete(t.nodes[n].after, i)
		if len(t.nodes[n].after) == 0 {
			fmt.Println("XXX released", n)
			heap.Push(t.todo, n)
		} else {
			fmt.Println("XXX cannot release", n, t.nodes[n].after)
		}
	}
}

func (t *topo) run() error {
	for {
		if t.todo.Len() > 0 {
			i := heap.Pop(t.todo).(int)
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

func (t *topo) processOne(i int, released bool) {
	fmt.Println("XXX popped", i, released)
	if i > len(t.funcs) {
		if released {
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
	_, out := fm.DownFlows()
	for _, t := range out {
		if num, ok := providedTypes[t]; ok {
			fmt.Println("XXX release down", t)
			t.release(num, i)
		}
	}
	receive, _ := fm.UpFlows()
	for _, t := range receive {
		if num, ok := receivedTypes[t]; ok {
			fmt.Println("XXX release up", t)
			t.release(num, i)
		}
	}
}

// Code below originated with the container/heap documentation
type IntHeap []int

func (h IntHeap) Len() int           { return len(h) }
func (h IntHeap) Less(i, j int) bool { return h[i] < h[j] }
func (h IntHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *IntHeap) Push(x interface{}) {
	// Push and Pop use pointer receivers because they modify the slice's length,
	// not just its contents.
	*h = append(*h, x.(int))
}

func (h *IntHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}
