package nvelope_test

import (
	"fmt"
	"io"
	"net/http/httptest"
	"strings"

	"github.com/gorilla/mux"
	"github.com/muir/nject/npoint"
	"github.com/muir/nject/nvelope"
)

func setupTestService(path string, f interface{}) func(url, body string) {
	return captureOutputFunc(func(i ...interface{}) {
		fmt.Println(i...)
	}, path, f)
}

func captureOutput(path string, f interface{}) func(url, body string) string {
	var o string
	do := captureOutputFunc(func(i ...interface{}) {
		o += fmt.Sprint(i...)
	}, path, f)
	return func(url, body string) string {
		o = ""
		do(url, body)
		return o
	}
}

func captureOutputFunc(out func(...interface{}), path string, f interface{}) func(url, body string) {
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
	client := ts.Client()

	return func(url string, body string) {
		// nolint:noctx
		res, err := client.Post(ts.URL+url, "application/json",
			strings.NewReader(body))
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
