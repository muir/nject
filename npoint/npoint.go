package npoint

// TODO: tests for CallsInner annotation
// TODO: inject path as a Path type.
// TODO: inject route as a mux.Route type.
// TODO: Duplicate service
// TODO: Duplicate endpoint
// TODO: When making copies of valueCollections, do deep copies when they implement a DeepCopy method.
// TODO: Re-use slots in the value collection when values do not overlap in time
// TODO: order the value collection so that middleware can make only partial copies
// TODO: new annotator: skip copying the value collection

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/BlueOwlOpenSource/nject/nject"
)

// Service allows a group of related endpoints to be started
// together. This form of service represents an already-started
// service that binds its enpoints using a simple binder like
// http.ServeMux.HandleFunc().
type Service struct {
	Name       string
	endpoints  map[string]*EndpointRegistration
	Collection *nject.Collection
	binder     EndpointBinder
	lock       sync.Mutex
}

// ServiceRegistration allows a group of related endpoints to be started
// together. This form of service represents pre-registered service
// service that binds its enpoints using a simple binder like
// http.ServeMux.HandleFunc().  None of the endpoints associated
// with this service will initialize themselves or start listening
// until Start() is called.
type ServiceRegistration struct {
	Name       string
	started    *Service
	endpoints  map[string]*EndpointRegistration
	Collection *nject.Collection
	lock       sync.Mutex
}

// EndpointRegistration holds endpoint defintions for services
// that will be started w/o gorilla mux.
type EndpointRegistration struct {
	finalFunc  http.HandlerFunc
	initialize func()
	path       string
	bound      bool
}

// PreregisterService creates a service that must be Start()ed later.
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
func PreregisterService(name string, funcs ...interface{}) *ServiceRegistration {
	return registerService(name, funcs...)
}

// RegisterService creates a service and starts it immediately.
func RegisterService(name string, binder EndpointBinder, funcs ...interface{}) *Service {
	sr := PreregisterService(name, funcs...)
	return sr.Start(binder)
}

func registerService(name string, funcs ...interface{}) *ServiceRegistration {
	return &ServiceRegistration{
		Name:       name,
		endpoints:  make(map[string]*EndpointRegistration),
		Collection: nject.Sequence(name, funcs...),
	}
}

func registerServiceWithMux(name string, funcs ...interface{}) *ServiceRegistrationWithMux {
	return &ServiceRegistrationWithMux{
		Name:       name,
		endpoints:  make(map[string][]*EndpointRegistrationWithMux),
		Collection: nject.Sequence(name, funcs...),
	}
}

// EndpointBinder is the signature of the binding function
// used to start a ServiceRegistration.
type EndpointBinder func(path string, fn http.HandlerFunc)

// Start runs all staticInjectors for all endpoints pre-registered with this
// service.  Bind all endpoints and starts listening.   Start() may
// only be called once.
func (s *ServiceRegistration) Start(binder EndpointBinder) *Service {
	s.lock.Lock()
	defer s.lock.Unlock()
	if s.started != nil {
		panic("duplicate call to Start()")
	}
	for path, endpoint := range s.endpoints {
		endpoint.start(path, binder)
	}
	svc := &Service{
		Name:       s.Name,
		endpoints:  s.endpoints,
		Collection: s.Collection,
		binder:     binder,
	}
	s.started = svc
	return svc
}

// Start and endpoint: invokes the endpoint and binds it to the
// path.
func (r *EndpointRegistration) start(path string, binder EndpointBinder) {
	r.path = path
	if !r.bound {
		r.initialize()
		r.bound = true
	}
	binder(path, r.finalFunc)
}

// CreateEndpoint generates a http.HandlerFunc from a list of handlers.  This bypasses Service,
// ServiceRegistration, ServiceWithMux, and ServiceRegistrationWithMux.  The
// static initializers are invoked immedately.
func CreateEndpoint(funcs ...interface{}) http.HandlerFunc {
	c := nject.Sequence("createEndpoint", funcs...)
	if len(funcs) == 0 {
		panic("at least one handler must be provided")
	}
	var httpHandler http.HandlerFunc
	var initFunc func()
	err := c.Bind(&httpHandler, &initFunc)
	if err != nil {
		panic(fmt.Sprintf("Cannot create HandlerFunc binding %s", nject.DetailedError(err)))
	}
	initFunc()
	return httpHandler
}

// RegisterEndpoint pre-registers an endpoint.  The provided funcs must all match one of the
// handler types.
// The functions provided are invoked in-order.
// Static injectors first and the endpoint last.
//
// The return value does not need to be retained -- it is also remembered
// in the ServiceRegistration.
//
// The endpoint initialization will not run until the service is started.  If the
// service has already been started, the endpoint will be started immediately.
func (s *ServiceRegistration) RegisterEndpoint(path string, funcs ...interface{}) *EndpointRegistration {
	s.lock.Lock()
	defer s.lock.Unlock()
	if s.endpoints[path] != nil {
		panic("endpoint path already registered")
	}
	r := &EndpointRegistration{
		path: path,
	}
	err := s.Collection.Append(path, funcs...).Bind(&r.finalFunc, &r.initialize)
	if err != nil {
		panic(fmt.Sprintf("Cannot bind %s %s: %s", s.Name, path, nject.DetailedError(err)))
	}
	s.endpoints[path] = r
	if s.started != nil {
		r.start(path, s.started.binder)
	}
	return r
}

// RegisterEndpoint registers and immedately starts an endpoint.
// The provided funcs must all match one of handler types.
// The functions provided are invoked in-order.
// Static injectors first and the endpoint last.
//
// The return value does not need to be retained -- it is also remembered
// in the Service.
func (s *Service) RegisterEndpoint(path string, funcs ...interface{}) *EndpointRegistration {
	s.lock.Lock()
	defer s.lock.Unlock()
	if s.endpoints[path] != nil {
		panic("endpoint path already registered")
	}
	r := &EndpointRegistration{
		path: path,
	}
	err := s.Collection.Append(path, funcs...).Bind(&r.finalFunc, &r.initialize)
	if err != nil {
		panic(fmt.Sprintf("Cannot bind %s %s: %s", s.Name, path, nject.DetailedError(err)))
	}
	s.endpoints[path] = r
	r.start(path, s.binder)
	return r
}
