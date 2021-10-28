package nvelope

import (
	"fmt"
	"runtime/debug"

	"github.com/muir/nject/nject"

	"github.com/pkg/errors"
)

// LogFlusher is used to check if a logger implements
// Flush().  This is useful as part of a panic handler.
type LogFlusher interface {
	Flush()
}

type panicError struct {
	msg   string
	r     interface{}
	stack string
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
		msg:   fmt.Sprint(r),
		r:     r,
		stack: string(debug.Stack()),
	}
	*ep = errors.WithStack(pe)
	log.Error("panic!", map[string]interface{}{
		"msg":   pe.msg,
		"stack": pe.stack,
	})
	if flusher, ok := log.(LogFlusher); ok {
		flusher.Flush()
	}
}

// CatchPanic is a wrapp that catches downstream panics and returns
// an error a downsteam provider panic's.
var CatchPanic = nject.Provide("catch-panic", catchPanicInjector)

func catchPanicInjector(inner func() error, log BasicLogger) (err error) {
	defer SetErrorOnPanic(&err, log)
	err = inner()
	return
}

// RecoverInterface returns the interface{} that recover()
// originally provided.  Or it returns nil if the
// error isn't a from a panic recovery.  This works only
// in conjunction with SetErrorOnPanic() and CatchPanic.
func RecoverInterface(err error) interface{} {
	if pe, ok := isPanicError(err); ok {
		return pe.r
	}
	return nil
}

// RecoverStack returns the stack from when recover()
// originally caught the panic.  Or it returns "" if the
// error isn't a from a panic recovery.  This works only
// in conjunction with SetErrorOnPanic() and CatchPanic.
func RecoverStack(err error) string {
	if pe, ok := isPanicError(err); ok {
		return pe.stack
	}
	return ""
}

func isPanicError(err error) (panicError, bool) {
	var pe panicError
	if errors.As(err, &pe) {
		return pe, true
	}
	return panicError{}, false
}
