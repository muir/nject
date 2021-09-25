package npoint_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"

	"github.com/gorilla/mux"
	"github.com/muir/nject/npoint"
)

// The npoint framework distinguishes parameters based on their types.
// All parameters of type "string" look the same, but a type that is
// defined as another type (like exampleType) is a different type.
type (
	exampleType    string
	fooParam       string
	fromMiddleware string
)

// exampleStaticInjector will not be called until the service.Start()
// call in Example_PreregisterServiceWithMux.  It will be called only
// once per endpoint registered.  Since it has a return value, it will
// only run if a downstream handler consumes the value it returns.
//
// The values returned by injectors and available as input parameters
// to any downstream handler.
func exampleStaticInjector() exampleType {
	return "example static value"
}

// exampleInjector will be called for each request.  We know that
// exampleInjector is a regular injector because it takes a parameter
// that is specific to the request (*http.Request).
func exampleInjector(r *http.Request) fooParam {
	return fooParam(r.FormValue("foo"))
}

type returnValue interface{}

// jsonifyResult wraps all handlers downstream of it in the call chain.
// We know that jsonifyResult is a middleware handler because its first
// argument is an function with an anonymous type (inner).   Calling inner
// invokes all handlers downstream from jsonifyResult.  The value returned
// by inner can come from the return values of the final endpoint handler
// or from values returned by any downstream middleware.  The parameters
// to inner are available as inputs to any downstream handler.
//
// Parameters are matched by their types.  Since inner returns a returnValue,
// it can come from any downstream middleware or endpoint that returns something
// of type returnValue.
func jsonifyResult(inner func(fromMiddleware) returnValue, w http.ResponseWriter) {
	v := inner("jsonify!")
	w.Header().Set("Content-Type", "application/json")
	encoded, _ := json.Marshal(v)
	w.WriteHeader(200)
	w.Write(encoded)
}

// Endpoints are grouped and started by services.  Handlers that are
// common to all endpoints are attached to the service.
var service = npoint.PreregisterServiceWithMux("example-service",
	exampleStaticInjector,
	jsonifyResult)

func init() {
	// The /example endpoint is bound to a handler chain
	// that combines the functions included at the service
	// level and the functions included here.  The final chain is:
	//	exampleStaticInjector, jsonifyResult, exampleInjector, exampleEndpoint.
	// ExampleStaticInjector and jsonifyResult come from the service
	// definition.  ExampleInjector and exampleEndpoint are attached when
	// the endpoint is registered.
	//
	// Handlers will execute in the order of the chain: exampleStaticInjector
	// then jsonifyResult.  When jsonifyResult calls inner(), exampleInjector
	// runs, then exampleEndpoint.   When exampleEndpoint returns, inner() returns
	// so jsonifyResult continues its work.  When jsonifyResult returns, the
	// handler chain is complete and the http server can form a reply from the
	// ResponseWriter.
	//
	// Since service is WithMux, we can use gorilla mux modifiers when
	// we register endpoints.  This allows us to trivially indicate that our
	// example endpoint supports the GET method only.
	service.RegisterEndpoint(
		"/example", exampleInjector, exampleEndpoint).Methods("GET")
}

// This is the final endpoint handler.  The parameters it takes can
// be provided by any handler upstream from it.  It can also take the two
// values that are included by the http handler signature: http.ResponseWriter
// and *http.Request.
//
// Any values that the final endpoint handler returns must be consumed by an
// upstream middleware handler.  In this example, a "returnValue" is returned
// here and consumed by jsonifyResult.
func exampleEndpoint(sv exampleType, foo fooParam, mid fromMiddleware) returnValue {
	return map[string]string{
		"value": fmt.Sprintf("%s-%s-%s", sv, foo, mid),
	}
}

// The code below puts up a test http server, hits the /example
// endpoint, decodes the response, prints it, and exits.  This
// is just to exercise the endpoint defined above.  The interesting
// stuff happens above.
func Example() {
	muxRouter := mux.NewRouter()
	service.Start(muxRouter)
	localServer := httptest.NewServer(muxRouter)
	defer localServer.Close()
	// nolint:noctx
	r, err := http.Get(localServer.URL + "/example?foo=bar")
	if err != nil {
		fmt.Println("get error", err)
		return
	}
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		fmt.Println("read error", err)
		return
	}
	r.Body.Close()
	var res map[string]string
	err = json.Unmarshal(buf, &res)
	if err != nil {
		fmt.Println("unmarshal error", err)
		return
	}
	fmt.Println("Value:", res["value"])
	// Output: Value: example static value-bar-jsonify!
}
