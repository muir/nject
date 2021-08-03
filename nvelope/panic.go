package nvelope

import (
	"fmt"

	"github.com/muir/nject/nject"
	"github.com/pkg/errors"
)

// LogFlusher is used to check if a logger implements
// Flush().  This is useful as part of a panic handler.
type LogFlusher interface {
	Flush()
}

type panicError struct {
	msg string
	r   interface{}
}

func (err panicError) Error() string {
	return "panic: " + err.msg
}

// SetErrorOnPanic should be called as a defer.  It
// sets an error value if there is a panic.
func SetErrorOnPanic(ep *error, log BasicLogger) {
	r := recover()
	if r == nil {
		return
	}
	pe := panicError{
		msg: fmt.Sprint(r),
		r:   r,
	}
	*ep = errors.WithStack(pe)
	log.Error(pe.msg)
	if flusher, ok := log.(LogFlusher); ok {
		flusher.Flush()
	}
}

var CatchPanics = nject.Provide("catch-panic", catchPanicInjector)

func catchPanicInjector(inner func() error, log BasicLogger) (err error) {
	defer SetErrorOnPanic(&err, log)
	err = inner()
	return
}

// PanicMessage returns the interface{} that recover()
// originally provided.  Or it returns nil if the
// error isn't a from a panic recovery
func PanicMessage(err error) interface{} {
	for {
		if pe, ok := err.(panicError); ok {
			return pe.r
		}
		if c, ok := err.(causer); ok {
			err = c.Cause()
			continue
		}
		if u, ok := err.(unwraper); ok {
			err = u.Unwrap()
			continue
		}
		return 500
	}
}
