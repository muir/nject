package npoint

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/BlueOwlOpenSource/nject/nject"
	"github.com/gorilla/mux"
)

// ServiceWithMux allows a group of related endpoints to be started
// together. This form of service represents an already-started
// service that binds its endpoints using gorilla
// mux.Router.HandleFunc.
type ServiceWithMux struct {
	Name       string
	endpoints  map[string][]*EndpointRegistrationWithMux
	Collection *nject.Collection
	binder     endpointBinderWithMux
	lock       sync.Mutex
}

// ServiceRegistrationWithMux allows a group of related endpoints to be started
// together. This form of service represents pre-registered service
// service that binds its endpoints using gorilla
// mux.Router.HandleFunc.  None of the endpoints associated
// with this service will initialize themselves or start listening
// until Start() is called.
type ServiceRegistrationWithMux struct {
	Name       string
	started    *ServiceWithMux
	endpoints  map[string][]*EndpointRegistrationWithMux
	Collection *nject.Collection
	lock       sync.Mutex
}

// EndpointRegistrationWithMux holds endpoint definitions for
// services that will be Start()ed with gorilla mux.  Most of
// the gorilla mux methods can be used with these endpoint
// definitions.
type EndpointRegistrationWithMux struct {
	EndpointRegistration
	muxroutes []func(*mux.Route) *mux.Route
	route     *mux.Route
	err       error
}

type endpointBinderWithMux func(string, func(http.ResponseWriter, *http.Request)) *mux.Route

// PreregisterServiceWithMux creates a service that must be Start()ed later.
//
// The passed in funcs follow the same rules as for the funcs in a
// nject.Collection.
//
// The injectors and middlware functions will precede any injectors
// and middleware specified on each endpoint that registers with this
// service.
//
// PreregsteredServices do not initialize or bind to handlers until
// they are Start()ed.
//
// The name of the service is just used for error messages and is otherwise ignored.
func PreregisterServiceWithMux(name string, funcs ...interface{}) *ServiceRegistrationWithMux {
	return registerServiceWithMux(name, funcs...)
}

// RegisterServiceWithMux creates a service and starts it immediately.
func RegisterServiceWithMux(name string, router *mux.Router, funcs ...interface{}) *ServiceWithMux {
	sr := PreregisterServiceWithMux(name, funcs...)
	return sr.Start(router)
}

// Start calls endpoints initializers for this Service and then registers all the
// endpoint handlers to the router.   Start() should be called at most once.
func (s *ServiceRegistrationWithMux) Start(router *mux.Router) *ServiceWithMux {
	s.lock.Lock()
	defer s.lock.Unlock()
	if s.started != nil {
		panic("duplicate call to Start()")
	}
	for path, el := range s.endpoints {
		for _, endpoint := range el {
			endpoint.start(path, router.HandleFunc)
		}
	}
	svc := &ServiceWithMux{
		Name:       s.Name,
		endpoints:  s.endpoints,
		Collection: s.Collection,
		binder:     router.HandleFunc,
	}
	s.started = svc
	return svc
}

// Start an endpoint: invokes the endpoint and binds it to the  path.
// If called more than once, subsequent calls to
// EndpointRegistrationWithMux methods that act on the route will
// only act on the last route bound.
func (r *EndpointRegistrationWithMux) start(
	path string,
	binder endpointBinderWithMux,
) *mux.Route {
	if !r.bound {
		r.initialize()
		r.bound = true
	}
	r.path = path
	r.route = binder(path, r.finalFunc)
	for _, mod := range r.muxroutes {
		r.route = mod(r.route)
	}
	r.err = r.route.GetError()
	return r.route
}

// RegisterEndpoint pre-registers an endpoint.  The provided funcs must all match one of the
// handler types.
// The functions provided are invoked in-order.
// Static injectors first and the endpoint last.
//
// The return value does not need to be retained -- it is also remembered
// in the Service.  The return value can be used to add mux.Route-like
// modifiers.  They will not take effect until the service is started.
//
// The endpoint initialization will not run until the service is started.  If the
// service has already been started, the endpoint will be started immediately.
func (s *ServiceRegistrationWithMux) RegisterEndpoint(path string, funcs ...interface{}) *EndpointRegistrationWithMux {
	s.lock.Lock()
	defer s.lock.Unlock()
	wmux := &EndpointRegistrationWithMux{
		EndpointRegistration: EndpointRegistration{
			path: path,
		},
		muxroutes: make([]func(*mux.Route) *mux.Route, 0),
	}
	err := s.Collection.Append(path, funcs...).Bind(&wmux.EndpointRegistration.finalFunc, &wmux.EndpointRegistration.initialize)
	if err != nil {
		panic(fmt.Sprintf("Cannot bind %s %s: %s", s.Name, path, nject.DetailedError(err)))
	}
	s.endpoints[path] = append(s.endpoints[path], wmux)

	if s.started != nil {
		wmux.start(path, s.started.binder)
	}
	return wmux
}

// RegisterEndpoint registers and immediately starts an endpoint.
// The provided funcs must all match one of the handler types.
// The functions provided are invoked in-order.
// Static injectors first and the endpoint last.
func (s *ServiceWithMux) RegisterEndpoint(path string, funcs ...interface{}) *mux.Route {
	s.lock.Lock()
	defer s.lock.Unlock()
	wmux := &EndpointRegistrationWithMux{
		EndpointRegistration: EndpointRegistration{
			path: path,
		},
		muxroutes: make([]func(*mux.Route) *mux.Route, 0),
	}
	err := s.Collection.Append(path, funcs...).Bind(&wmux.EndpointRegistration.finalFunc, &wmux.EndpointRegistration.initialize)
	if err != nil {
		panic(fmt.Sprintf("Cannot bind %s %s: %s", s.Name, path, nject.DetailedError(err)))
	}
	s.endpoints[path] = append(s.endpoints[path], wmux)
	return wmux.start(path, s.binder)
}
