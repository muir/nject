package nvelope_test

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/gorilla/mux"
	"github.com/muir/nject/npoint"
	"github.com/muir/nject/nvelope"
)

func main() {
	r := mux.NewRouter()
	srv := &http.Server{
		Addr:    "0.0.0.0:8080",
		Handler: r,
	}
	Service(r)
	log.Fatal(srv.ListenAndServe())
}

type PostBodyModel struct {
	Use      string `json:"use"`
	Exported string `json:"exported"`
	Names    string `json:"names"`
}

type ExampleRequestBundle struct {
	Request     PostBodyModel `nvelope:"model"`
	With        string        `nvelope:"path,name=with"`
	Parameters  int64         `nvelope:"path,name=parameters"`
	Friends     []int         `nvelope:"query,name=friends"`
	ContentType string        `nvelope:"header,name=Content-Type"`
}

type ExampleResponse struct {
	Stuff string `json:"stuff"`
	Here  string `json:"here"`
}

func HandleExampleEndpoint(req ExampleRequestBundle) (nvelope.Response, error) {
	fmt.Println("XXX handle example endpoint called")
	if req.ContentType != "application/json" {
		return nil, errors.New("content type must be application/json")
	}
	fmt.Println("XXX returning response")
	return ExampleResponse{
		Stuff: "something useful",
	}, nil
}

func Service(router *mux.Router) {
	service := npoint.RegisterServiceWithMux("example", router)
	service.RegisterEndpoint("/a/path/{with}/{parameters}",
		// order matters and this is the correct order
		nvelope.LoggerFromStd(log.Default()),
		nvelope.InjectWriter,
		nvelope.EncodeJSON,
		nvelope.CatchPanics,
		nvelope.Nil204,
		nvelope.ReadBody,
		nvelope.DecodeJSON,
		HandleExampleEndpoint,
	).Methods("POST")
}

// Example shows an injection chain handling a single endpoint using nject,
// npoint, and nvelope.
func Example() {
	r := mux.NewRouter()
	Service(r)
	ts := httptest.NewServer(r)
	client := ts.Client()
	res, err := client.Post(ts.URL+"/a/path/joe/37.3", "application/json",
		strings.NewReader(`{"Use":"yeah","Exported":"uh hu"}`))
	fmt.Println("response error", err)
	if err != nil {
		return
	}
	b, err := io.ReadAll(res.Body)
	fmt.Println("read body error", err)
	fmt.Println("response:", string(b))
	// Output: response error <nil>
	// read body error <nil>
	// response: {"stuff":"something useful"}

}
