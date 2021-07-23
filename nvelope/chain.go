package nvelope

import (
	"encoding/json"
	"net/http"

	"github.com/muir/nject/nject"
)

var InjectWriter = nject.Provide("writer", NewDeferredWriter())

var InjectLogger = nject.Provide("logger", nject.Loose(func(r *http.Request, w DeferredWriter) {
}))

type Return interface{}

type Logger interface {
	Error(msg string, fields ...map[string]interface{})
	Warn(msg string, fields ...map[string]interface{})
}

var JSON = MakeEncoder("JSON", json.Marshal)

func MakeEncoder(name string, marshaller func(interface{}) ([]byte, error)) nject.Provider {
	return nject.Provide("marshal-"+name,
		func(
			inner func() (Any, error),
			w DeferredWriter,
			log Logger,
			r *http.Request,
		) {
			model, err := inner()
			if w.Done() {
				return
			}
			if err != nil {
				model = err
			}
			enc, err := marshaller(model)
			if err != nil {
				w.WriteHeader(500)
				w.Write([]byte(fmt.Sprintf("Cannot marshal model: %s", err)))
				log.Error("Cannot marshal response",
					map[string]interface{}{
						"error":  err.Error(),
						"method": r.Method,
						"uri":    r.URL.String(),
					})
				return
			}
			if e, ok := model.(error); ok {
				w.WriteHeader(GetReturnCode(e))
			}
			err = w.Write()
			if err != nil {
				log.Warn("Cannot write response",
					map[string]interface{}{
						"error":  err.Error(),
						"method": r.Method,
						"uri":    r.URL.String(),
					})
			}
		})
}
