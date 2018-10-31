package nject

import (
	"sync/atomic"
	"testing"
)

func debugOn(t *testing.T) {
	debugOutputMu.Lock()
	debuglnHook = func(stuff ...interface{}) {
		t.Log(stuff...)
	}
	debugfHook = func(format string, stuff ...interface{}) {
		t.Logf(format+"\n", stuff...)
	}
	debugOutputMu.Unlock()
	atomic.StoreUint32(&debug, 1)
}

func debugOff() {
	debugOutputMu.Lock()
	debuglnHook = nil
	debugfHook = nil
	debugOutputMu.Unlock()
	atomic.StoreUint32(&debug, 0)
}

func wrapTest(t *testing.T, inner func(*testing.T)) {
	if !t.Run("1st attempt", func(t *testing.T) { inner(t) }) {
		t.Run("2nd attempt", func(t *testing.T) {
			debugOn(t)
			defer debugOff()
			inner(t)
		})
	}
}
