package nvelope_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/muir/nject/npoint"
	"github.com/muir/nject/nvelope"

	"github.com/gorilla/mux"
)

func RequestTimingMiddlewareHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("timing start")
		before := time.Now()
		next.ServeHTTP(w, r)
		after := time.Now()
		duration := after.Sub(before)
		fmt.Println("timing end, Request took", duration.Round(time.Hour))
	})
}

func AuthenticationMiddlewareHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("authentication start")
		a := r.Header.Get("Authentication")
		if a != "good" {
			w.WriteHeader(401)
			w.Write([]byte("Invalid authentication"))
			fmt.Println("authentication end (failed)")
			return
		}
		next.ServeHTTP(w, r)
		fmt.Println("authentication end")
	})
}

func AuthorizationMiddlewareHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("authorization start")
		vars := mux.Vars(r)
		if vars["with"] != "john" {
			w.WriteHeader(403)
			w.Write([]byte("Invalid authorization"))
			fmt.Println("authorization end (failed)")
			return
		}
		next.ServeHTTP(w, r)
		fmt.Println("authorization end")
	})
}

func ServiceWithMiddlewareHandler(router *mux.Router) {
	service := npoint.RegisterServiceWithMux("example", router)
	service.RegisterEndpoint("/a/path/{with}/{parameters}",
		// order matters and this is a correct order
		nvelope.MiddlewareHandlerBaseWriter(RequestTimingMiddlewareHandler),
		nvelope.NoLogger,
		nvelope.InjectWriter,
		nvelope.AutoFlushWriter, // because middlware won't Flush()
		nvelope.MiddlewareHandlerDeferredWriter(AuthenticationMiddlewareHandler, AuthorizationMiddlewareHandler),
		nvelope.EncodeJSON,
		nvelope.CatchPanic,
		func() (nvelope.Response, error) {
			fmt.Println("thing")
			return "did a thing", nil
		},
	).Methods("GET")
}

// Example shows an injection chain handling a single endpoint using nject,
// npoint, and nvelope.
func ExampleServiceWithMiddlewareHandler() {
	r := mux.NewRouter()
	ServiceWithMiddleware(r)
	ts := httptest.NewServer(r)
	client := ts.Client()
	doGet := func(url string, authHeader string) {
		req, err := http.NewRequestWithContext(context.Background(), "GET", ts.URL+url, nil)
		if err != nil {
			fmt.Println("request error:", err)
			return
		}
		req.Header.Set("Authentication", authHeader)
		res, err := client.Do(req)
		if err != nil {
			fmt.Println("response error:", err)
			return
		}
		b, err := ioutil.ReadAll(res.Body)
		if err != nil {
			fmt.Println("read error:", err)
			return
		}
		res.Body.Close()
		fmt.Println(res.StatusCode, "->"+string(b))
	}
	doGet("/a/path/john/37", "good")
	doGet("/a/path/john/37", "bad")
	doGet("/a/path/fred/37", "good")
	// Output: timing start
	// authentication start
	// authorization start
	// thing
	// authorization end
	// authentication end
	// timing end, Request took 0s
	// 200 ->"did a thing"
	// timing start
	// authentication start
	// authentication end (failed)
	// timing end, Request took 0s
	// 401 ->Invalid authentication
	// timing start
	// authentication start
	// authorization start
	// authorization end (failed)
	// authentication end
	// timing end, Request took 0s
	// 403 ->Invalid authorization
}
