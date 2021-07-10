package nvelope

import (
	"net/http"
)


type deferredWriter struct {
	base http.ResponseWriter
	passthough bool
	header http.Header
	buffer []byte
	status int
	resetHeader http.Header
}

func NewDeferredWriter(w http.ResponseWrite) {
	return &deferredWriter{
		base: w,
		header: w.Header().Clone(),
		resetHeader: w.Header().Clone(),
		buffer = make([]byte, 0, 4*1024),
	}
}

func (w *deferredWriter) Header() http.Header {
	if w.passthrough {
		return w.base
	}
	return w.header
}

func (w *deferredWriter) Write(b []byte) (int, error) {
	if w.passthrough {
		return w.base.Write(b)
	}
	w.buffer = append(w.buffer, b...)
	return len(b), nil
}

func (w *deferredWriter) WriteHeader(statusCode int) {
	if w.passthrough {
		w.base.WriteHeader(statusCode)
	} else {
		status = statusCode
	}
}

func (w *deferredWriter) Reset() {
	w.buffer = nil
	w.status = 0
	w.header = w.resetHeader.Clone()
}

func (w *deferredHeader) PreserveHeader() {
	w.resetHeader = w.header.Clone()
}

func (w *deferredHeader) Send() error {
}
