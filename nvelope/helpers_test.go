package nvelope_test

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"

	"github.com/muir/nject/npoint"
	"github.com/muir/nject/nvelope"

	"github.com/gorilla/mux"
)

// nolint:deadcode,unused
func setupTestService(path string, f interface{}) func(string, ...mod) {
	return captureOutputFunc(func(i ...interface{}) {
		fmt.Println(i...)
	}, path, f)
}

func captureOutput(path string, f interface{}) func(string, ...mod) string {
	var o string
	do := captureOutputFunc(func(i ...interface{}) {
		o += fmt.Sprint(i...)
	}, path, f)
	return func(url string, mods ...mod) string {
		o = ""
		do(url, mods...)
		return o
	}
}

type mod func(*http.Request, *http.Client, *httptest.Server)

// nolint:unused,deadcode
func body(s string) mod {
	return func(r *http.Request, cl *http.Client, ts *httptest.Server) {
		r.Body = ioutil.NopCloser(strings.NewReader(s))
	}
}

func cookie(k, v string) mod {
	return func(r *http.Request, cl *http.Client, ts *httptest.Server) {
		cl.Jar.SetCookies(r.URL, []*http.Cookie{
			{Name: k, Value: v},
		})
	}
}

func header(k, v string) mod {
	return func(r *http.Request, cl *http.Client, ts *httptest.Server) {
		r.Header[k] = append(r.Header[k], v)
	}
}

func captureOutputFunc(out func(...interface{}), path string, f interface{}) func(string, ...mod) {
	router := mux.NewRouter()
	service := npoint.RegisterServiceWithMux("example", router)
	service.RegisterEndpoint(path,
		// order matters and this is a correct order
		nvelope.NoLogger,
		nvelope.InjectWriter,
		nvelope.EncodeJSON,
		nvelope.CatchPanic,
		nvelope.Nil204,
		nvelope.ReadBody,
		nvelope.DecodeJSON,
		f,
	).Methods("POST")
	ts := httptest.NewServer(router)

	return func(url string, mods ...mod) {
		client := ts.Client()
		var err error
		client.Jar, err = cookiejar.New(&cookiejar.Options{})
		if err != nil {
			panic("jar")
		}
		// nolint:noctx
		req, err := http.NewRequest("POST", ts.URL+url, ioutil.NopCloser(strings.NewReader("")))
		if err != nil {
			panic("request")
		}
		for _, m := range mods {
			m(req, client, ts)
		}

		// nolint:noctx
		res, err := client.Do(req)
		if err != nil {
			out("response error:", err)
			return
		}
		b, err := io.ReadAll(res.Body)
		if err != nil {
			out("read error:", err)
			return
		}
		res.Body.Close()
		out(res.StatusCode, "->"+string(b))
	}
}
