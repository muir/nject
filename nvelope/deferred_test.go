package nvelope_test

import (
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/muir/nject/nvelope"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testResponseWriter struct {
	header             http.Header
	simulateWriteError error
	buffer             []byte
	code               int
}

var _ http.ResponseWriter = &testResponseWriter{}

func (w *testResponseWriter) Header() http.Header  { return w.header }
func (w *testResponseWriter) WriteHeader(code int) { w.code = code }
func (w *testResponseWriter) Write(b []byte) (int, error) {
	if w.simulateWriteError != nil {
		// nolint:errorlint
		if w.simulateWriteError == io.ErrShortWrite {
			if len(b) == 0 {
				return 0, nil
			}
			w.buffer = append(w.buffer, b[0])
			w.simulateWriteError = nil
			return 1, io.ErrShortWrite
		}
		return 0, w.simulateWriteError
	}
	w.buffer = append(w.buffer, b...)
	return len(b), nil
}

func TestUnderlyingWriter(t *testing.T) {
	tw := &testResponseWriter{header: make(http.Header)}
	w := nvelope.NewDeferredWriter(tw)
	assert.Equal(t, tw, w.UnderlyingWriter())
}

func TestFlush(t *testing.T) {
	tw := &testResponseWriter{header: make(http.Header)}
	tw.Header().Set("a", "b")
	w := nvelope.NewDeferredWriter(tw)
	w.Write([]byte("howdy"))
	assert.Empty(t, tw.buffer, "no write before flush")
	assert.Equal(t, "b", w.Header().Get("a"), "original header still there")
	w.Header().Set("c", "d")
	assert.Equal(t, "", tw.Header().Get("c"), "original header untouched with new key")
	w.Header().Set("a", "d")
	assert.Equal(t, "", tw.Header().Get("c"), "original header untouched with existing key")
	assert.Equal(t, "d", w.Header().Get("c"), "new header override works though")
	w.WriteHeader(303)
	assert.Equal(t, 0, tw.code, "code not written before flush")
	assert.False(t, w.Done(), "done before flush")
	require.NoError(t, w.Flush(), "flush")
	assert.True(t, w.Done(), "done after flush")
	assert.Equal(t, "howdy", string(tw.buffer), "write after flush")
	assert.Equal(t, 303, tw.code, "code written after flush")
	assert.Equal(t, "d", tw.Header().Get("c"), "new header written - c")
	assert.Equal(t, "d", tw.Header().Get("a"), "new header written - a")
}

func TestReset(t *testing.T) {
	tw := &testResponseWriter{header: make(http.Header)}
	tw.Header().Set("a", "b")
	w := nvelope.NewDeferredWriter(tw)

	w.Write([]byte("doody"))
	w.Header().Set("c", "e")
	w.Header().Set("a", "e")
	w.Header().Set("d", "g")
	w.WriteHeader(109)

	w.Reset()

	w.Write([]byte("howdy"))
	w.Header().Set("c", "d")
	w.Header().Set("a", "d")
	w.WriteHeader(303)

	require.NoError(t, w.Flush(), "flush")

	assert.Equal(t, "howdy", string(tw.buffer), "write after flush")
	assert.Equal(t, 303, tw.code, "code written after flush")
	assert.Equal(t, "d", tw.Header().Get("c"), "new header written - c")
	assert.Equal(t, "d", tw.Header().Get("a"), "new header written - a")
	assert.Equal(t, "", tw.Header().Get("d"), "new header not written - d")
}

func TestFlushErrShortWrite(t *testing.T) {
	tw := &testResponseWriter{header: make(http.Header)}
	w := nvelope.NewDeferredWriter(tw)

	tw.simulateWriteError = io.ErrShortWrite
	w.Write([]byte("howdy"))

	require.NoError(t, w.Flush(), "flush")
	assert.Equal(t, "howdy", string(tw.buffer), "write after flush")
}

func TestFlushError(t *testing.T) {
	tw := &testResponseWriter{header: make(http.Header)}
	w := nvelope.NewDeferredWriter(tw)

	tw.simulateWriteError = fmt.Errorf("an error")
	w.Write([]byte("howdy"))

	assert.Error(t, w.Flush(), "flush error")
}

func TestPreserveHeader(t *testing.T) {
	tw := &testResponseWriter{header: make(http.Header)}
	tw.Header().Set("a", "b")
	tw.Header().Set("b", "c")
	w := nvelope.NewDeferredWriter(tw)

	w.Header().Set("a", "B")
	w.Header().Set("c", "d")

	w.PreserveHeader()

	w.Reset()
	w.Header().Set("a", "x")
	w.Header().Set("d", "x")

	w.Reset()
	w.Flush()

	assert.Equal(t, "B", tw.Header().Get("a"), "new header written - a")
	assert.Equal(t, "c", tw.Header().Get("b"), "new header written - b")
	assert.Equal(t, "d", tw.Header().Get("c"), "new header written - c")
	assert.Equal(t, "", tw.Header().Get("d"), "new header written - d")
}
