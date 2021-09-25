package npoint_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/muir/nject/nject"
	"github.com/muir/nject/npoint"
	"github.com/stretchr/testify/assert"
)

var (
	// Use a custom Transport because httptest "helpfully" kills idle
	// connections on the default transport when a httptest server shuts
	// down.
	tr = &http.Transport{
		// Disable keepalives to avoid the hassle of closing idle
		// connections after each test.
		DisableKeepAlives: true,
	}
	client = &http.Client{Transport: tr}
)

func TestStaticInitializerWaitsForStart(t *testing.T) {
	t.Parallel()
	var debugOutput string
	var svcInitCount int
	var svcInvokeCount int
	var serviceSequenceInitFunc = func(db *nject.Debugging) string {
		t.Logf("service init chain")
		debugOutput = strings.Join(db.Included, "\n")
		svcInitCount++
		return "foo"
	}
	var serviceSequenceNonCacheable = func(s string, r *http.Request, db *nject.Debugging) {
		t.Logf("service invoke chain")
		debugOutput = strings.Join(db.Included, "\n")
		svcInvokeCount++
	}
	var initCount int
	var invokeCount int
	var endpointSequenceInitFunc = func(db *nject.Debugging) int {
		t.Logf("endpoint init chain")
		initCount++
		debugOutput = strings.Join(db.Included, "\n")
		return initCount
	}
	var endpointSequenceFinalFunc = func(w http.ResponseWriter, i int, db *nject.Debugging) {
		t.Logf("endpoint invoke chain")
		w.WriteHeader(204)
		debugOutput = strings.Join(db.Included, "\n")
		invokeCount++
	}
	multiStartups(
		t,
		"test",
		nject.Sequence("SERVICE", nject.Cacheable(serviceSequenceInitFunc), nject.Cacheable(serviceSequenceNonCacheable)),
		nject.Sequence("ENDPOINT", nject.Cacheable(endpointSequenceInitFunc), nject.Cacheable(endpointSequenceFinalFunc)),
		func(s string) {
			// reset
			t.Logf("reset for %s", s)
			debugOutput = ""
			initCount = 0
			invokeCount = 0
			svcInitCount = 10
			svcInvokeCount = 10
		},
		func(s string) {
			// after register
			assert.Equal(t, 0, initCount, s+" after register endpoint init count\n"+debugOutput)
			assert.Equal(t, 0, invokeCount, s+" after register endpoint invoke count\n"+debugOutput)
			assert.Equal(t, 10, svcInitCount, s+" after register service init count\n"+debugOutput)
			assert.Equal(t, 10, svcInvokeCount, s+" after register service invoke count\n"+debugOutput)
		},
		func(s string) {
			// after start
			assert.Equal(t, 1, initCount, s+" after start endpoint init count\n"+debugOutput)
			assert.Equal(t, 0, invokeCount, s+" after start endpoint invoke count\n"+debugOutput)
			assert.Equal(t, 11, svcInitCount, s+" after start service init count\n"+debugOutput)
			assert.Equal(t, 10, svcInvokeCount, s+" after start service invoke count\n"+debugOutput)
		},
		func(s string) {
			// after 1st call
			assert.Equal(t, 1, initCount, s+" 1st call start init count\n"+debugOutput)
			assert.Equal(t, 1, invokeCount, s+" 1st call start invoke count\n"+debugOutput)
			assert.Equal(t, 11, svcInitCount, s+" 1st call service init count\n"+debugOutput)
			assert.Equal(t, 11, svcInvokeCount, s+" 1st call service invoke count\n"+debugOutput)
		},
		func(s string) {
			// after 2nd call
			assert.Equal(t, 1, initCount, s+" 2nd call endpoint init count\n"+debugOutput)
			assert.Equal(t, 2, invokeCount, s+" 2nd call endpoint invoke count\n"+debugOutput)
			assert.Equal(t, 11, svcInitCount, s+" 2nd call service init count\n"+debugOutput)
			assert.Equal(t, 12, svcInvokeCount, s+" 2nd call service invoke count\n"+debugOutput)
		},
	)
}

func TestFallibleInjectorFailing(t *testing.T) {
	t.Parallel()
	var initCount int
	var invokeCount int
	var errorsCount int
	multiStartups(
		t,
		"test",
		nil,
		nject.Sequence("hc",
			func(inner func() error, w http.ResponseWriter) {
				t.Logf("wraper (before)")
				err := inner()
				t.Logf("wraper (after, err=%v)", err)
				if err != nil {
					assert.Equal(t, "bailing out", err.Error())
					errorsCount++
					w.WriteHeader(204)
				}
			},
			func() (nject.TerminalError, int) {
				t.Logf("endpoint init")
				initCount++
				return fmt.Errorf("bailing out"), initCount
			},
			func(w http.ResponseWriter, i int) error {
				t.Logf("endpoint invoke")
				w.WriteHeader(204)
				invokeCount++
				return nil
			},
		),
		func(s string) {
			// reset
			t.Logf("reset for %s", s)
			initCount = 0
			invokeCount = 0
			errorsCount = 0
		},
		func(s string) {
			// after register
			assert.Equal(t, 0, initCount, s+" after register endpoint init count")
			assert.Equal(t, 0, invokeCount, s+" after register endpoint invoke count")
			assert.Equal(t, 0, errorsCount, s+" after register endpoint invoke count")
		},
		func(s string) {
			// after start
			assert.Equal(t, 0, initCount, s+" after start endpoint init count")
			assert.Equal(t, 0, invokeCount, s+" after start endpoint invoke count")
			assert.Equal(t, 0, errorsCount, s+" after register endpoint invoke count")
		},
		func(s string) {
			// after 1st call
			assert.Equal(t, 1, initCount, s+" 1st call start init count")
			assert.Equal(t, 0, invokeCount, s+" 1st call start invoke count")
			assert.Equal(t, 1, errorsCount, s+" after register endpoint invoke count")
		},
		func(s string) {
			// after 2nd call
			assert.Equal(t, 2, initCount, s+" 2nd call endpoint init count")
			assert.Equal(t, 0, invokeCount, s+" 2nd call endpoint invoke count")
			assert.Equal(t, 2, errorsCount, s+" after register endpoint invoke count")
		},
	)
}

func TestFallibleInjectorNotFailing(t *testing.T) {
	t.Parallel()
	var initCount int
	var invokeCount int
	var errorsCount int
	multiStartups(
		t,
		"testFallibleInjectorNotFailing",
		nil,
		nject.Sequence("hc",
			func(inner func() error) {
				t.Logf("wraper (before)")
				err := inner()
				t.Logf("wraper (after, err=%v)", err)
				if err != nil {
					errorsCount++
				}
			},
			func() (nject.TerminalError, int) {
				t.Logf("endpoint init")
				initCount++
				return nil, 17
			},
			func(w http.ResponseWriter, i int) error {
				assert.Equal(t, 17, i)
				t.Logf("endpoint invoke")
				w.WriteHeader(204)
				invokeCount++
				return nil
			},
		),
		func(s string) {
			// reset
			t.Logf("reset for %s", s)
			initCount = 0
			invokeCount = 0
			errorsCount = 0
		},
		func(s string) {
			// after register
			assert.Equal(t, 0, initCount, s+" after register endpoint init count")
			assert.Equal(t, 0, invokeCount, s+" after register endpoint invoke count")
			assert.Equal(t, 0, errorsCount, s+" after register endpoint invoke count")
		},
		func(s string) {
			// after start
			assert.Equal(t, 0, initCount, s+" after start endpoint init count")
			assert.Equal(t, 0, invokeCount, s+" after start endpoint invoke count")
			assert.Equal(t, 0, errorsCount, s+" after register endpoint invoke count")
		},
		func(s string) {
			// after 1st call
			assert.Equal(t, 1, initCount, s+" 1st call start init count")
			assert.Equal(t, 1, invokeCount, s+" 1st call start invoke count")
			assert.Equal(t, 0, errorsCount, s+" after register endpoint invoke count")
		},
		func(s string) {
			// after 2nd call
			assert.Equal(t, 2, initCount, s+" 2nd call endpoint init count")
			assert.Equal(t, 2, invokeCount, s+" 2nd call endpoint invoke count")
			assert.Equal(t, 0, errorsCount, s+" after register endpoint invoke count")
		},
	)
}

func multiStartups(
	t *testing.T,
	name string,
	shc *nject.Collection,
	hc *nject.Collection,
	reset func(string),
	afterRegister func(string), // not called for CreateEnpoint
	afterStart func(string),
	afterCall1 func(string),
	afterCall2 func(string),
) {
	{
		n := name + "-1PreregisterNoMuxRegisterEndpointBeforeStart"
		reset(n)
		ept := "/" + n
		s := npoint.PreregisterService(n, shc)
		s.RegisterEndpoint(ept, hc)
		afterRegister(n)
		b := NewBinder()
		s.Start(b.Bind)
		afterStart(n)
		// nolint:bodyclose
		b.Call(ept, "GET", "", nil)
		afterCall1(n)
		// nolint:bodyclose
		b.Call(ept, "GET", "", nil)
		afterCall2(n)
	}
	{
		n := name + "-2PreregisterNoMuxRegisterEndpointAfterStartWithOriginalService"
		reset(n)
		ept := "/" + n
		s := npoint.PreregisterService(n, shc)
		b := NewBinder()
		s.Start(b.Bind)
		s.RegisterEndpoint(ept, hc)
		// afterRegister(n)
		afterStart(n)
		// nolint:bodyclose
		b.Call(ept, "GET", "", nil)
		afterCall1(n)
		// nolint:bodyclose
		b.Call(ept, "GET", "", nil)
		afterCall2(n)
	}
	{
		n := name + "-3PreregisterNoMuxRegisterEndpointAfterStartWithStartedService"
		reset(n)
		ept := "/" + n
		s := npoint.PreregisterService(n, shc)
		b := NewBinder()
		sr := s.Start(b.Bind)
		sr.RegisterEndpoint(ept, hc)
		// afterRegister(n)
		afterStart(n)
		// nolint:bodyclose
		b.Call(ept, "GET", "", nil)
		afterCall1(n)
		// nolint:bodyclose
		b.Call(ept, "GET", "", nil)
		afterCall2(n)
	}
	{
		n := name + "-4PreregisterServiceWithMuxRegisterEndpointBeforeStart"
		reset(n)
		ept := "/" + n
		s := npoint.PreregisterServiceWithMux(n, shc)
		s.RegisterEndpoint(ept, hc)
		afterRegister(n)
		muxRouter := mux.NewRouter()
		s.Start(muxRouter)
		localServer := httptest.NewServer(muxRouter)
		defer localServer.Close()
		afterStart(n)
		t.Logf("GET %s%s\n", localServer.URL, ept)
		// nolint:noctx
		resp, err := client.Get(localServer.URL + ept)
		assert.NoError(t, err)
		if resp != nil {
			resp.Body.Close()
		}
		afterCall1(n)
		// nolint:noctx
		resp, err = client.Get(localServer.URL + ept)
		assert.NoError(t, err, name)
		if resp != nil {
			assert.Equal(t, 204, resp.StatusCode, name)
			resp.Body.Close()
		}
		afterCall2(n)
	}
	{
		n := name + "-5PreregisterWithMuxRegisterEndpointAfterStartUsingOriginalService"
		reset(n)
		ept := "/" + n
		s := npoint.PreregisterServiceWithMux(n, shc)
		muxRouter := mux.NewRouter()
		s.Start(muxRouter)
		localServer := httptest.NewServer(muxRouter)
		defer localServer.Close()
		s.RegisterEndpoint(ept, hc)
		// afterRegister(n)
		afterStart(n)
		t.Logf("GET %s%s\n", localServer.URL, ept)
		// nolint:noctx
		resp, err := client.Get(localServer.URL + ept)
		assert.NoError(t, err)
		if resp != nil {
			resp.Body.Close()
		}
		afterCall1(n)
		// nolint:noctx
		resp, err = client.Get(localServer.URL + ept)
		assert.NoError(t, err, name)
		if resp != nil {
			assert.Equal(t, 204, resp.StatusCode, name)
			resp.Body.Close()
		}
		afterCall2(n)
	}
	{
		n := name + "-6PreregisterWithMuxRegisterEndpointAfterStartUsingStartedService"
		reset(n)
		ept := "/" + n
		s := npoint.PreregisterServiceWithMux(n, shc)
		muxRouter := mux.NewRouter()
		sr := s.Start(muxRouter)
		localServer := httptest.NewServer(muxRouter)
		defer localServer.Close()
		sr.RegisterEndpoint(ept, hc)
		// afterRegister(n)
		afterStart(n)
		t.Logf("GET %s%s\n", localServer.URL, ept)
		// nolint:noctx
		resp, err := client.Get(localServer.URL + ept)
		assert.NoError(t, err)
		if resp != nil {
			resp.Body.Close()
		}
		afterCall1(n)
		// nolint:noctx
		resp, err = client.Get(localServer.URL + ept)
		assert.NoError(t, err, name)
		if resp != nil {
			assert.Equal(t, 204, resp.StatusCode, name)
			resp.Body.Close()
		}
		afterCall2(n)
	}
	{
		n := name + "-7CreateEndpoint"
		reset(n)
		ept := "/" + n
		b := NewBinder()
		e := npoint.CreateEndpoint(shc, hc)
		b.Bind(ept, e)
		// afterRegister(n)
		afterStart(n)
		// nolint:bodyclose
		b.Call(ept, "GET", "", nil)
		afterCall1(n)
		// nolint:bodyclose
		b.Call(ept, "GET", "", nil)
		afterCall2(n)
	}
	{
		n := name + "-8RegisterServiceNoMux"
		reset(n)
		ept := "/" + n
		b := NewBinder()
		s := npoint.RegisterService(n, b.Bind, shc)
		s.RegisterEndpoint(ept, hc)
		// afterRegister(n)
		afterStart(n)
		// nolint:bodyclose
		b.Call(ept, "GET", "", nil)
		afterCall1(n)
		// nolint:bodyclose
		b.Call(ept, "GET", "", nil)
		afterCall2(n)
	}
	{
		n := name + "-9RegisterServiceWithMux"
		reset(n)
		ept := "/" + n
		muxRouter := mux.NewRouter()
		s := npoint.RegisterServiceWithMux(n, muxRouter, shc)
		s.RegisterEndpoint(ept, hc)
		// afterRegister(n)
		localServer := httptest.NewServer(muxRouter)
		defer localServer.Close()
		afterStart(n)
		t.Logf("GET %s%s\n", localServer.URL, ept)
		// nolint:noctx
		resp, err := client.Get(localServer.URL + ept)
		assert.NoError(t, err)
		if resp != nil {
			resp.Body.Close()
		}
		afterCall1(n)
		// nolint:noctx
		resp, err = client.Get(localServer.URL + ept)
		assert.NoError(t, err, name)
		if resp != nil {
			assert.Equal(t, 204, resp.StatusCode, name)
			resp.Body.Close()
		}
		afterCall2(n)
	}
}

func TestMuxModifiers(t *testing.T) {
	t.Parallel()
	s := npoint.PreregisterServiceWithMux("TestCharacterize")

	s.RegisterEndpoint("/x", func(w http.ResponseWriter) {
		w.WriteHeader(204)
	}).Methods("GET")

	s.RegisterEndpoint("/x", func(w http.ResponseWriter) {
		w.WriteHeader(205)
	}).Methods("POST")

	muxRouter := mux.NewRouter()
	s.Start(muxRouter)

	localServer := httptest.NewServer(muxRouter)
	defer localServer.Close()

	// nolint:noctx
	resp, err := client.Get(localServer.URL + "/x")
	if !assert.NoError(t, err) {
		return
	}
	resp.Body.Close()
	assert.Equal(t, 204, resp.StatusCode)

	// nolint:noctx
	resp, err = client.Post(localServer.URL+"/x", "application/json", nil)
	if !assert.NoError(t, err) {
		return
	}
	resp.Body.Close()
	assert.Equal(t, 205, resp.StatusCode)
}
