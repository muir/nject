package npoint_test

import (
	"bytes"
	"database/sql"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/muir/nject/nject"
	"github.com/muir/nject/npoint"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
)

type intType3 int
type intType5 int
type intType7 int

type stringA string
type stringB string
type stringC string
type stringD string
type stringE string
type stringF string

func NewBinder() *ManualBinder {
	return &ManualBinder{
		Bound: make(map[string]http.HandlerFunc),
	}
}

type ManualBinder struct {
	Bound map[string]http.HandlerFunc
}

func (b *ManualBinder) Bind(path string, h http.HandlerFunc) {
	b.Bound[path] = h
}
func (b *ManualBinder) Call(path string, method string, buf string, h http.Header) *http.Response {
	handler, found := b.Bound[path]
	if !found {
		panic(fmt.Sprintf("no handler for %s", path))
	}
	url := "http://localhost" + path
	req, err := http.NewRequest(method, url, bytes.NewReader([]byte(buf)))
	if err != nil {
		panic(err)
	}
	req.Header = h
	w := NewWriter()
	handler(w, req)
	resp := &http.Response{
		Status:        fmt.Sprintf("%d Something", w.code),
		StatusCode:    w.code,
		Proto:         "HTTP/1.0",
		ProtoMajor:    1,
		ProtoMinor:    0,
		Header:        w.h,
		Body:          ioutil.NopCloser(bytes.NewBuffer([]byte(w.buf))),
		ContentLength: int64(len(w.buf)),
		Request:       req,
	}
	return resp
}

type Writer struct {
	h    http.Header
	buf  string
	code int
}

func NewWriter() *Writer                      { return &Writer{h: make(http.Header)} }
func (w *Writer) Write(b []byte) (int, error) { w.buf += string(b); return len(b), nil }
func (w *Writer) Header() http.Header         { return w.h }
func (w *Writer) WriteHeader(i int)           { w.code = i }

func TestTestFramework(t *testing.T) {
	t.Parallel()
	b := NewBinder()
	var bodyReceived string
	var headerReceived string
	b.Bind("/y", func(w http.ResponseWriter, r *http.Request) {
		buf, err := ioutil.ReadAll(r.Body)
		assert.NoError(t, err)
		bodyReceived = string(buf)
		headerReceived = r.Header.Get("X-Test-Request")
		w.Header().Set("X-Test-Respond", "H1")
		w.Write([]byte("some data written"))
		w.WriteHeader(203)
	})
	h := make(http.Header)
	h.Set("X-Test-Request", "H2")
	resp := b.Call("/y", "POST", "some data sent", h)
	buf, err := ioutil.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.Equal(t, "some data written", string(buf))
	assert.Equal(t, "H1", resp.Header.Get("X-Test-Respond"))
	assert.Equal(t, "", resp.Header.Get("X-Test-Request"))
	assert.Equal(t, "some data sent", bodyReceived)
	assert.Equal(t, "H2", headerReceived)
}

type interfaceI interface {
	I() int
}
type interfaceJ interface {
	I() int
}
type interfaceK interface {
	I() int
}
type doesI struct {
	i int
}

func (di *doesI) I() int { return di.i * 2 }

type doesJ struct {
	j int
}

func (dj *doesJ) I() int { return dj.j * 3 }

func TestVariablePassing(t *testing.T) {
	t.Skip("Requires fuzzy matching")
	t.Parallel()
	s := npoint.PreregisterService("TestVariablePassing")
	s.RegisterEndpoint("/x",
		// ------------- static injectors ------------
		// [0] Send simple things
		func() (stringA, stringB) {
			return "static1", "static2"
		},
		func(b stringB) {
			assert.Equal(t, stringB("static2"), b)
		},
		func(b stringB) {
			assert.Equal(t, stringB("static2"), b)
		},

		// [3] Send interfaces
		func() interfaceI {
			return &doesI{7}
		},
		func() interfaceJ {
			return &doesI{8}
		},
		func() *doesI {
			return &doesI{9}
		},
		func() *doesJ {
			return &doesJ{10}
		},

		// [7] check receiving by priority
		func(i interfaceI) {
			// exact match on interface
			assert.Equal(t, 14, i.I())
		},
		func(j interfaceJ) {
			// exact match on interface
			assert.Equal(t, 16, j.I())
		},
		func(di *doesI) {
			// exact match on interface
			assert.Equal(t, 18, di.I())
		},
		func(k interfaceK) {
			// nearest one that satisfies interface
			assert.Equal(t, 30, k.I())
		},

		// ------------- middleware ------------
		// [11]
		func(inner func(intType3, stringC) (intType5, stringE), a stringA) {
			assert.Equal(t, stringA("static1"), a)
			i5, e := inner(93, "c-v")
			assert.Equal(t, stringE("fooE"), e)
			assert.Equal(t, intType5(55), i5)
		},

		// ------------- injector ------------
		// [12]
		func(c stringC, i3 intType3, j *doesJ) stringA {
			assert.Equal(t, stringC("c-v"), c)
			assert.Equal(t, intType3(93), i3)
			assert.Equal(t, 30, j.I())
			return "newAv"
		},

		// ------------- endpoint ------------
		func(i3 intType3, b stringB, a stringA, c stringC) (stringE, intType5) {
			assert.Equal(t, intType3(93), i3)
			assert.Equal(t, stringB("static2"), b)
			assert.Equal(t, stringA("newAv"), a)
			assert.Equal(t, stringC("c-v"), c)
			return "fooE", 55
		})
	b := NewBinder()
	s.Start(b.Bind)
	_ = b.Call("/x", "GET", "", make(http.Header))
}

var inclusionTests = []struct {
	name     string
	endpoint interface{}
	called   string
}{
	{
		"just e",
		func(e stringE) {},
		"e cd",
	},
}

func TestInclusion(t *testing.T) {
	t.Parallel()

	for _, tc := range inclusionTests {
		s := npoint.PreregisterService(fmt.Sprintf("TestDemandInclusion-%s", tc.name))
		called := make(map[string]bool)
		s.RegisterEndpoint("/x",
			func() (stringA, stringB) {
				called["ab"] = true
				return "static1", "static2"
			},
			func() (stringC, stringD) {
				called["cd"] = true
				return "static3", "static4"
			},
			func(d stringD) stringE {
				called["e"] = true
				return "static5"
			},
			func(b stringB) stringF {
				called["f"] = true
				return "static6"
			},
			tc.endpoint)
		b := NewBinder()
		s.Start(b.Bind)
		_ = b.Call("/x", "GET", "", make(http.Header))
		expected := make(map[string]bool)
		for _, c := range strings.Split(tc.called, " ") {
			expected[c] = true
		}
		assert.Equal(t, expected, called, tc.name)
	}
}

type error2 struct {
	e error
}

func (e2 error2) Error() string {
	return e2.e.Error()
}

type paymentProvider interface {
	Stuff() int
}
type examplePaymentProvider int

func (epp examplePaymentProvider) Stuff() int {
	return int(epp * 2)
}

type tripsURI string
type csettings map[string]string
type logger interface {
	Logf(string, ...interface{})
}
type enhancedWriter interface {
	http.ResponseWriter
	S(int)
}
type enhancedWriterImp struct {
	http.ResponseWriter
}

func (w enhancedWriterImp) S(i int) {
	w.WriteHeader(i)
}

type dbname string
type rbody []byte
type jresult interface{}

func TestChains(t *testing.T) {
	t.Parallel()
	var chainTests = []struct {
		Name   string
		Panics bool
		Chain  []interface{}
	}{
		{
			"interface games",
			true, // requires fuzzy matching
			[]interface{}{
				func() logger {
					return t
				},
				func(w http.ResponseWriter, l logger) enhancedWriter {
					return &enhancedWriterImp{w}
				},
				func(inner func() jresult) {
					j := inner()
					e, is := j.(error)
					assert.True(t, is)
					assert.Equal(t, "example error", e.Error())
				},
				func(inner func() (jresult, error2), l logger, w enhancedWriter) jresult {
					_, err := inner()
					return err
				},
				func(inner func() error, l logger, w enhancedWriter) error2 {
					return error2{inner()}
				},
				func() error {
					return fmt.Errorf("example error")
				},
			},
		},
		{
			"unused return",
			true,
			[]interface{}{
				func(inner func() error) error {
					return inner()
				},
				func(r *http.Request, w http.ResponseWriter) error {
					return nil
				},
			},
		},
		{
			"obscured return",
			true,
			[]interface{}{
				func(inner func() error2) {
					_ = inner()
				},
				func(inner func() error) error {
					return inner()
				},
				func(inner func() error) *error2 {
					return &error2{inner()}
				},
				func(r *http.Request, w http.ResponseWriter) error {
					return nil
				},
			},
		},
		{
			"regression",
			false,
			[]interface{}{
				nject.Sequence("service",
					func() paymentProvider {
						return examplePaymentProvider(7)
					},
					func() tripsURI {
						return "tu"
					},
					func() context.Context {
						return context.Background()
					},
					nject.Sequence("common-handlers",
						func() logger {
							return t
						},
						nject.Sequence("base-collection",
							func() csettings {
								return make(map[string]string)
							},
							func(r *http.Request, l logger) logger {
								return l
							},
							func(w http.ResponseWriter, l logger) enhancedWriter {
								return &enhancedWriterImp{w}
							},
							func(w enhancedWriter, ac csettings) {},
							func(inner func() jresult, w enhancedWriter, l logger) {
								_ = inner()
							},
							func(inner func() (jresult, error2), l logger, w enhancedWriter) jresult {
								res, _ := inner()
								return res
							},
							func(inner func() error, l logger, w enhancedWriter) error2 {
								return error2{inner()}
							},
							func(inner func(rbody) error, r *http.Request) error {
								return inner([]byte("foo"))
							},
						),
						nject.Sequence("open-database",
							func() dbname {
								return "foo"
							},
							func(inner func(*sql.DB) error, name dbname) error {
								db, err := sql.Open("postgres", string(name))
								if err != nil {
									return err
								}
								err = inner(db)
								db.Close()
								return err
							},
						),
						func(inner func(*sql.Tx) error, db *sql.DB, l logger, w enhancedWriter) error {
							tx, _ := db.Begin()
							return inner(tx)
						},
					),
				),
				func(inner func() error, w enhancedWriter) jresult {
					return nil
				},
				func(l logger, b rbody, tx *sql.Tx) error {
					return nil
				},
			},
		},
		{
			"static regression",
			false,
			[]interface{}{
				nject.Sequence("service",
					func() paymentProvider {
						return examplePaymentProvider(7)
					},
					func() tripsURI {
						return "tu"
					},
					func() context.Context {
						return context.Background()
					},
					nject.Sequence("common-handlers",
						func() logger {
							return t
						},
						nject.Sequence("base-collection",
							func() csettings {
								return make(map[string]string)
							},
							func(r *http.Request, l logger) logger {
								return l
							},
							func(w http.ResponseWriter, l logger) enhancedWriter {
								return &enhancedWriterImp{w}
							},
							func(w enhancedWriter, ac csettings) {},
							func(inner func() jresult, w enhancedWriter, l logger) {
								_ = inner()
							},
							func(inner func() (jresult, error2), l logger, w enhancedWriter) jresult {
								res, _ := inner()
								return res
							},
							func(inner func() error, l logger, w enhancedWriter) error2 {
								return error2{inner()}
							},
							func(inner func(rbody) error, r *http.Request) error {
								return inner([]byte("foo"))
							},
						),
						nject.Sequence("open-database",
							func() dbname {
								return "foo"
							},
							func(inner func(*sql.DB) error, name dbname) error {
								db, err := sql.Open("postgres", string(name))
								if err != nil {
									return err
								}
								err = inner(db)
								db.Close()
								return err
							},
						),
						func(inner func(*sql.Tx) error, db *sql.DB, l logger, w enhancedWriter) error {
							tx, _ := db.Begin()
							return inner(tx)
						},
						func() intType7 {
							return 3
						},
					),
				),
				nject.Sequence("endpoint-list",
					func(i intType7) intType5 {
						return 3
					},
					func(l logger, b rbody, tx *sql.Tx, i intType5) (jresult, error) {
						return nil, nil
					},
				),
			},
		},
	}

	for _, test := range chainTests {
		t.Log("TEST:", test.Name)
		f := func() {
			e := npoint.CreateEndpoint(test.Chain...)
			b := NewBinder()
			b.Bind("/foo", e)
			b.Call("/foo", "GET", "", nil)
		}
		if test.Panics {
			assert.Panics(t, f, test.Name)
		} else {
			f()
			assert.NotPanics(t, f, test.Name)
		}
	}
}
