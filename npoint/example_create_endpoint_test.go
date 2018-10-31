package npoint_test

import (
	"database/sql"
	"net/http"

	"github.com/BlueOwlOpenSource/nject/nject"
	"github.com/BlueOwlOpenSource/nject/npoint"
)

// CreateEndpoint is the simplest way to start using the npoint framework.  It
// generates an http.HandlerFunc from a list of handlers.  The handlers will be called
// in order.   In the example below, first WriteErrorResponse() will be called.  It
// has an inner() func that it uses to invoke the rest of the chain.  When
// WriteErrorResponse() calls its inner() function, the db injector returned by
// InjectDB is called.  If that does not return error, then the inline function below
// to handle the endpint is called.
func ExampleCreateEndpoint() {
	mux := http.NewServeMux()
	mux.HandleFunc("/my/endpoint", npoint.CreateEndpoint(
		WriteErrorResponse,
		InjectDB("postgres", "postgres://..."),
		func(r *http.Request, db *sql.DB, w http.ResponseWriter) error {
			// Write response to w or return error...
			return nil
		}))
}

// WriteErrorResponse invokes the remainder of the handler chain by calling inner().
func WriteErrorResponse(inner func() nject.TerminalError, w http.ResponseWriter) {
	err := inner()
	if err != nil {
		w.Write([]byte(err.Error()))
		w.WriteHeader(500)
	}
}

// InjectDB returns a handler function that opens a database connection.   If the open
// fails, executation of the handler chain is terminated.
func InjectDB(driver, uri string) func() (nject.TerminalError, *sql.DB) {
	return func() (nject.TerminalError, *sql.DB) {
		db, err := sql.Open(driver, uri)
		if err != nil {
			return err, nil
		}
		return nil, db
	}
}
