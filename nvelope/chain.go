package nvelope

import (
	"net/http"

	"github.com/muir/nject/nject"
)

var InjectWriter = nject.Provide("writer", NewDeferredWriter())

var InjectLogger = nject.Provide("logger", nject.Loose(func(r *http.Request, w DeferredWriter) {
	
type Any interface{}

var JSON = nject.Provide("marshal-JSON", func(inner() Any, w DeferredWriter) {
	model := inner()
	if w.Done() {
		return
	}
