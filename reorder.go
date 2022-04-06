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

func reorder(funcs []*provider) []*provider {
	receivedTypes := make(map[reflect.Type]int)
	providedTypes := make(map[reflect.Type]int)

	counter := len(funcs) + 1

	pairs := make([][2]int, 0, len(funcs)*6)
	aAfterB := func(i, j int) {
		if i == -1 || j == -1 {
			return
		}
		pairs = append(pairs, [2]int{i, j})
	}
	excluded := make([]*provider, 0, len(funcs))
	prior := -1
	for i, fm := range funcs {
		if !fm.include {
			excluded = append(excluded, fm)
			continue
		}
		if !fm.reorder {
			aAfterB(i, prior)
			prior = i
		}
		in, _ := fm.DownFlows()
		for _, t := range in {
			if num, ok := providedTypes[t]; ok {
				aAfterB(i, num)
			} else {
				providedTypes[t] = counter
				aAfterB(i, counter)
				counter++
			}
		}
		_, produce := fm.UpFlows()
		for _, t := range produce {
			if num, ok := receivedTypes[t]; ok {
				aAfterB(i, num)
			} else {
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
		nodes[pair[0]].before[pair[1]] = struct{}{}
		nodes[pair[1]].after[pair[0]] = struct{}{}
	}

	var underlying IntHeap
	for i, node := range nodes {
		if len(node.after) == 0 && len(node.before) > 0 {
			underlying = append(underlying, i)
		}
	}
	todo := &underlying
	heap.Init(todo)

	release := func(n, i int) {
		if n >= len(funcs) {
			heap.Push(todo, n)
		} else {
			delete(nodes[n].after, i)
			if len(nodes[n].after) == 0 {
				heap.Push(todo, n)
			}
		}
	}
	reorderedFuncs := make([]*provider, 0, len(funcs))
	for todo.Len() > 0 {
		i := heap.Pop(todo).(int)
		for n := range nodes[i].before {
			release(n, i)
		}
		nodes[i].before = nil
		if i < len(funcs) {
			fm := funcs[i]
			for len(excluded) > 0 && fm.originalPosition > excluded[0].originalPosition {
				reorderedFuncs = append(reorderedFuncs, excluded[0])
				excluded = excluded[1:]
			}
			reorderedFuncs = append(reorderedFuncs, fm)
			_, out := fm.DownFlows()
			for _, t := range out {
				if num, ok := providedTypes[t]; ok {
					release(num, i)
				}
			}
			receive, _ := fm.UpFlows()
			for _, t := range receive {
				if num, ok := receivedTypes[t]; ok {
					release(num, i)
				}
			}
		}
	}
	reorderedFuncs = append(reorderedFuncs, excluded...)

	if len(reorderedFuncs) < len(funcs) {
		for i := 0; i < len(funcs); i++ {
			if len(nodes[i].after) > 0 {
				funcs[i].cannotInclude = fmt.Errorf("member of dependency cycle")
				reorderedFuncs = append(reorderedFuncs, funcs[i])
			}
		}
	}
	reorderedFuncs = append(reorderedFuncs)
	return reorderedFuncs
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
