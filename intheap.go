package nject

import (
	"container/heap"
)

// Code below originated with the container/heap documentation

type IntsHeap [][2]int

func (h IntsHeap) Len() int           { return len(h) }
func (h IntsHeap) Less(i, j int) bool { return h[i][0] < h[j][0] }
func (h IntsHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *IntsHeap) Push(x interface{}) {
	// Push and Pop use pointer receivers because they modify the slice's length,
	// not just its contents.
	*h = append(*h, x.([2]int))
}

func (h *IntsHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

func push(h *IntsHeap, funcs []*provider, i int) {
	priority := i
	if i < len(funcs) && funcs[i].reorder {
		priority -= len(funcs)
	}
	heap.Push(h, [2]int{priority, i})
}

func pop(h *IntsHeap) int {
	//nolint:errcheck // we trust the type
	x := heap.Pop(h).([2]int)
	return x[1]
}
