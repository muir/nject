package nserve

import (
	"sync"
	"sync/atomic"
)

var hookCounter int32

type hookOrder string

const (
	ForwardOrder hookOrder = "forward"
	ReverseOrder           = "forward"
)

type hookId int32

// Hook is the handle/name for a list of callbacks to invoke.
type Hook struct {
	Id            hookId
	lock          sync.Mutex
	Name          string
	Order         hookOrder
	InvokeOnError []*Hook
	ContinuePast  bool
	ErrorCombiner func(first, second error) error
}

// Copy makes a deep copy of a hook and the new hook gets a new Id.
// Copy is thread-safe.
func (h *Hook) Copy() *Hook {
	h.lock.Lock()
	defer h.lock.Unlock()
	oe := make([]*Hook, len(h.InvokeOnError))
	copy(oe, h.InvokeOnError)
	hc := *h
	hc.InvokeOnError = oe
	hc.Id = hookId(atomic.AddInt32(&hookCounter, 1))
	return &hc
}

// NewHook creates a new category of callbacks.
func NewHook(name string, order hookOrder) *Hook {
	return &Hook{
		Id:    hookId(atomic.AddInt32(&hookCounter, 1)),
		Name:  name,
		Order: order,
	}
}

// OnError adds to the set of hooks to invoke when this hook is
// thows an error.  Call with nil to clear the set of hooks to invoke.
// OnError is thread-safe.
func (h *Hook) OnError(e *Hook) *Hook {
	h.lock.Lock()
	defer h.lock.Unlock()
	if e == nil {
		h.InvokeOnError = nil
	} else {
		h.InvokeOnError = append(h.InvokeOnError, e)
	}
	return h
}

// SetErrorCombiner sets a function to combine two errors into one when there
// is more than one error to return from a invoking all the callbacks
// SetErrorCombiner is thread-safe.
func (h *Hook) SetErrorCombiner(f func(first, second error) error) *Hook {
	h.lock.Lock()
	defer h.lock.Unlock()
	h.ErrorCombiner = f
	return h
}

// ContinuePastError sets if callbacks should continue to be invoked
// if there has already been an error.
// ContinuePastError is thread-safe.
func (h *Hook) ContinuePastError(b bool) *Hook {
	h.lock.Lock()
	defer h.lock.Unlock()
	h.ContinuePast = b
	return h
}

// String is not thread-safe with respect to reaching into a hook and
// changing it's Name.  Don't do that.
func (h *Hook) String() string {
	return "hook " + h.Name
}

var Shutdown = NewHook("shutdown", ReverseOrder)
var Stop = NewHook("stop", ReverseOrder).OnError(Shutdown).ContinuePastError(true)
var Start = NewHook("start", ForwardOrder).OnError(Stop)
