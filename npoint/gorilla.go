package npoint

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/gorilla/mux"
)

func (r *EndpointRegistrationWithMux) add(f func(m *mux.Route) *mux.Route) {
	r.muxroutes = append(r.muxroutes, f)
}

// Route returns the *mux.Route that has been registered to this endpoint, if possible.
func (r *EndpointRegistrationWithMux) Route() (*mux.Route, error) {
	if !r.bound {
		return nil, fmt.Errorf("Registration is not complete for %s", r.path)
	}
	if r.route == nil {
		return nil, fmt.Errorf("No *mux.Route was used to start %s", r.path)
	}
	return r.route, nil
}

// BuildOnly applies the mux.Route method of the same name to this endpoint when the endpoint is initialized.
func (r *EndpointRegistrationWithMux) BuildOnly() *EndpointRegistrationWithMux {
	r.add(func(m *mux.Route) *mux.Route { return m.BuildOnly() })
	return r
}

// BuildVarsFunc applies the mux.Route method of the same name to this endpoint when the endpoint is initialized.
func (r *EndpointRegistrationWithMux) BuildVarsFunc(f mux.BuildVarsFunc) *EndpointRegistrationWithMux {
	r.add(func(m *mux.Route) *mux.Route { return m.BuildVarsFunc(f) })
	return r
}

// Headers applies the mux.Route method of the same name to this endpoint when the endpoint is initialized.
func (r *EndpointRegistrationWithMux) Headers(pairs ...string) *EndpointRegistrationWithMux {
	r.add(func(m *mux.Route) *mux.Route { return m.Headers(pairs...) })
	return r
}

// HeadersRegexp applies the mux.Route method of the same name to this endpoint when the endpoint is initialized.
func (r *EndpointRegistrationWithMux) HeadersRegexp(pairs ...string) *EndpointRegistrationWithMux {
	r.add(func(m *mux.Route) *mux.Route { return m.HeadersRegexp(pairs...) })
	return r
}

// Host applies the mux.Route method of the same name to this endpoint when the endpoint is initialized.
func (r *EndpointRegistrationWithMux) Host(tpl string) *EndpointRegistrationWithMux {
	r.add(func(m *mux.Route) *mux.Route { return m.Host(tpl) })
	return r
}

// MatcherFunc applies the mux.Route method of the same name to this endpoint when the endpoint is initialized.
func (r *EndpointRegistrationWithMux) MatcherFunc(f mux.MatcherFunc) *EndpointRegistrationWithMux {
	r.add(func(m *mux.Route) *mux.Route { return m.MatcherFunc(f) })
	return r
}

// Methods applies the mux.Route method of the same name to this endpoint when the endpoint is initialized.
func (r *EndpointRegistrationWithMux) Methods(methods ...string) *EndpointRegistrationWithMux {
	r.add(func(m *mux.Route) *mux.Route { return m.Methods(methods...) })
	return r
}

// Name applies the mux.Route method of the same name to this endpoint when the endpoint is initialized.
func (r *EndpointRegistrationWithMux) Name(name string) *EndpointRegistrationWithMux {
	r.add(func(m *mux.Route) *mux.Route { return m.Name(name) })
	return r
}

// Path applies the mux.Route method of the same name to this endpoint when the endpoint is initialized.
func (r *EndpointRegistrationWithMux) Path(tpl string) *EndpointRegistrationWithMux {
	r.add(func(m *mux.Route) *mux.Route { return m.Path(tpl) })
	return r
}

// PathPrefix applies the mux.Route method of the same name to this endpoint when the endpoint is initialized.
func (r *EndpointRegistrationWithMux) PathPrefix(tpl string) *EndpointRegistrationWithMux {
	r.add(func(m *mux.Route) *mux.Route { return m.PathPrefix(tpl) })
	return r
}

// Queries applies the mux.Route method of the same name to this endpoint when the endpoint is initialized.
func (r *EndpointRegistrationWithMux) Queries(pairs ...string) *EndpointRegistrationWithMux {
	r.add(func(m *mux.Route) *mux.Route { return m.Queries(pairs...) })
	return r
}

// Schemes applies the mux.Route method of the same name to this endpoint when the endpoint is initialized.
func (r *EndpointRegistrationWithMux) Schemes(schemes ...string) *EndpointRegistrationWithMux {
	r.add(func(m *mux.Route) *mux.Route { return m.Schemes(schemes...) })
	return r
}

// GetError calls the mux.Route method of the same name on the route created for this endpoint.
func (r *EndpointRegistrationWithMux) GetError() error {
	return r.err
}

// GetHandler calls the mux.Route method of the same name on the route created for this endpoint.
func (r *EndpointRegistrationWithMux) GetHandler() http.Handler {
	return r.route.GetHandler()
}

// GetHostTemplate calls the mux.Route method of the same name on the route created for this endpoint.
func (r *EndpointRegistrationWithMux) GetHostTemplate() (string, error) {
	return r.route.GetHostTemplate()
}

// GetName calls the mux.Route method of the same name on the route created for this endpoint.
func (r *EndpointRegistrationWithMux) GetName() string {
	return r.route.GetName()
}

// GetPathTemplate calls the mux.Route method of the same name on the route created for this endpoint.
func (r *EndpointRegistrationWithMux) GetPathTemplate() (string, error) {
	return r.route.GetPathTemplate()
}

// Match calls the mux.Route method of the same name on the route created for this endpoint.
func (r *EndpointRegistrationWithMux) Match(req *http.Request, match *mux.RouteMatch) bool {
	return r.route.Match(req, match)
}

// SkipClean calls the mux.Route method of the same name on the route created for this endpoint.
func (r *EndpointRegistrationWithMux) SkipClean() bool {
	return r.route.SkipClean()
}

// URL calls the mux.Route method of the same name on the route created for this endpoint.
func (r *EndpointRegistrationWithMux) URL(pairs ...string) (*url.URL, error) {
	return r.route.URL(pairs...)
}

// URLHost calls the mux.Route method of the same name on the route created for this endpoint.
func (r *EndpointRegistrationWithMux) URLHost(pairs ...string) (*url.URL, error) {
	return r.route.URLHost(pairs...)
}

// URLPath calls the mux.Route method of the same name on the route created for this endpoint.
func (r *EndpointRegistrationWithMux) URLPath(pairs ...string) (*url.URL, error) {
	return r.route.URLPath(pairs...)
}
