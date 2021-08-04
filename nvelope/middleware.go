package nvelope

import (
	"net/http"

	"github.com/muir/nject/nject"
)

// MiddlewareBaseWriter acts as a translator.  In the Go world, there
// are a bunch of packages that expect to use the wrapping
// middleware pattern.  The middleware pattern is harder to
// use and not as expressive as the patterns supported by
// npoint and nvelope, but there may be code written
// with the middleware pattern that you want to use with
// npoint and nvelope.
//
// MiddlewareBaseWriter converts existing middleware functions so that
// they're compatible with nject.  Because Middleware may wrap
// http.ResponseWriter, it should be used earlier in the injection
// chain than InjectWriter so that InjectWriter gets the already-wrapped
// http.ResponseWriter.
func MiddlewareBaseWriter(m ...middleware) nject.Provider {
	combined := combineMiddleware(m)

	return nject.Required(nject.Provide("wrapped-middleware-base",
		func(inner func(w http.ResponseWriter, r *http.Request), w http.ResponseWriter, r *http.Request) {
			combined(inner)(w, r)
		}))
}

// MiddlewareDeferredWriter acts as a translator.  In the Go world, there
// are a bunch of packages that expect to use the wrapping
// middleware pattern.  The middleware pattern is harder to
// use and not as expressive as the patterns supported by
// npoint and nvelope, but there may be code written
// with the middleware pattern that you want to use with
// npoint and nvelope.
//
// MiddlewareDeferredWriter converts existing middleware functions so that
// they're compatible with nject.  MiddlewareDeferredWriter injects a
// DeferredWriter into the the middleware handler chain.  If the chain
// replaces the writer, there will be two writers in play at once and
// results may be inconsistent.  MiddlewareDeferredWriter must be used
// after InjectWriter.
func MiddlewareDeferredWriter(m ...middleware) nject.Provider {
	combined := combineMiddleware(m)

	return nject.Required(nject.Provide("wrapped-middleware-deferred",
		func(inner func(w http.ResponseWriter, r *http.Request), w *DeferredWriter, r *http.Request) {
			combined(inner)(http.ResponseWriter(w), r)
		}))
}

type middleware func(http.HandlerFunc) http.HandlerFunc

func combineMiddleware(m []middleware) middleware {
	switch len(m) {
	case 0:
		return func(h http.HandlerFunc) http.HandlerFunc {
			return h
		}
	case 1:
		return m[0]
	default:
		combined := m[len(m)-1]
		for i := len(m) - 2; i >= 0; i-- {
			f := m[i]
			c := combined
			combined = func(h http.HandlerFunc) http.HandlerFunc {
				return f(c(h))
			}
		}
		return combined
	}
}
