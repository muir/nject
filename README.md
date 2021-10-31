# nject, npoint, nserve, & nvelope - dependency injection 

[![GoDoc](https://godoc.org/github.com/muir/nject?status.png)](https://pkg.go.dev/github.com/muir/nject)
![unit tests](https://github.com/muir/nject/actions/workflows/go.yml/badge.svg)
[![report card](https://goreportcard.com/badge/github.com/muir/nject)](https://goreportcard.com/report/github.com/muir/nject)
[![codecov](https://codecov.io/gh/muir/nject/branch/main/graph/badge.svg)](https://codecov.io/gh/muir/nject)
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fmuir%2Fnject.svg?type=shield)](https://app.fossa.com/projects/git%2Bgithub.com%2Fmuir%2Fnject?ref=badge_shield)


Install:

	go get github.com/muir/nject

---

This is a quartet of packages that together make up a most of a
golang API server framework:

nject: type safe dependency injection w/o requiring type assertions.

npoint: dependency injection wrappers for binding http endpoint handlers

nvelope: injection chains for building endpoints

nserve: injection chains for for starting and stopping servers

### Basic idea

Dependencies are injected via a call chain: list functions to be called
that take and return various parameters.  The functions will be called
in order using the return values from earlier functions as parameters
for later functions.

Parameters are identified by their types.  To have two different int
parameters, define custom types.

Type safety is checked before any functions are called.

Functions whose outputs are not used are not called.  Functions may
"wrap" the rest of the list so that they can choose to invoke the
remaing list zero or more times.

Chains may be pre-compiled into closures so that they have very little
runtime penealty.

### nject example

	func example() {
		// Sequences can be reused.
		providerChain := Sequence("example sequence",
			// Constants can be injected.
			"a literal string value",
			// This function will be run if something downstream needs an int
			func(s string) int {
				return len(s)
			})
		Run("example",
			providerChain,
			// The last function in the list is always run.  This one needs
			// and int and a string.  The string can come from the constant
			// and the int from the function in the provider chain.
			func(i int, s string) {
				fmt.Println(i, len(s))
			})
	}

### npoint example

CreateEndpoint is the simplest way to start using the npoint framework.  It
generates an http.HandlerFunc from a list of handlers.  The handlers will be called
in order.   In the example below, first WriteErrorResponse() will be called.  It
has an inner() func that it uses to invoke the rest of the chain.  When 
WriteErrorResponse() calls its inner() function, the db injector returned by
InjectDB is called.  If that does not return error, then the inline function below
to handle the endpint is called.  

	mux := http.NewServeMux()
	mux.HandleFunc("/my/endpoint", npoint.CreateEndpoint(
		WriteErrorResponse,
		InjectDB("postgres", "postgres://..."),
		func(r *http.Request, db *sql.DB, w http.ResponseWriter) error {
			// Write response to w or return error...
			return nil
		}))

WriteErrorResponse invokes the remainder of the handler chain by calling inner().

	func WriteErrorResponse(inner func() nject.TerminalError, w http.ResponseWriter) {
		err := inner()
		if err != nil {
			w.Write([]byte(err.Error()))
			w.WriteHeader(500)
		}
	}

InjectDB returns a handler function that opens a database connection.   If the open
fails, executation of the handler chain is terminated.  InjectDB returns an injector
so that it can be called with arguments -- injectors are functions, not invocations
and so we need to return a function.  InjectDB also closes the database connection.

	func InjectDB(driver, uri string) func(func(*sql.DB) error) error {
		return func(inner func(*sql.DB) error) (finalError error) {
			db, err := sql.Open(driver, uri)
			if err != nil {
				return err
			}
			defer func() {
				err := db.Close()
				if err != nil && finalError == nil {
					finalError = err
				}
			}()
			return inner(db)
		}
	}

### nvelope example

Nvelope provides pre-defined handlers for basic endpoint tasks.  When used
in combination with npoint, all that's left is the business logic.

```go
type ExampleRequestBundle struct {
	Request     PostBodyModel `nvelope:"model"`
	With        string        `nvelope:"path,name=with"`
	Parameters  int64         `nvelope:"path,name=parameters"`
	Friends     []int         `nvelope:"query,name=friends"`
	ContentType string        `nvelope:"header,name=Content-Type"`
}

func Service(router *mux.Router) {
	service := npoint.RegisterServiceWithMux("example", router)
	service.RegisterEndpoint("/some/path",
		nvelope.LoggerFromStd(log.Default()),
		nvelope.InjectWriter,
		nvelope.EncodeJSON,
		nvelope.CatchPanic,
		nvelope.Nil204,
		nvelope.ReadBody,
		nvelope.DecodeJSON,
		func (req ExampleRequestBundle) (nvelope.Response, error) {
			....
		},
	).Methods("POST")
}
```

### nserve example

On thing you might want to do with nserve is to use a `Hook` to trigger
per-library database migrations using [libschema](https://github.com/muir/libschema).

First create the hook:

```go
package myhooks

import "github.com/nject/nserve"

var MigrateMyDB = nserve.NewHook("migrate, nserve.Ascending)
```

In each library, have a create function:

```go
package users

import(
	"github.com/muir/libschema/lspostgres"
	"github.com/muir/nject/nserve"
)

func NewUsersStore(app *nserve.App) *Store {
	...
	app.On(myhooks.MigrateMyDB, func(database *libschema.Database) {
		database.Migrations("MyLibrary",
			lspostgres.Script("create users", `
				CREATE TABLE users (
					id	bigint PRIMARY KEY,
					name	text
				)
			`),
		)
	})
	...
	return &Store{}
}
```

Then as part of server startup, invoke the migration hook:

```go
package main

import(
	"github.com/muir/libschema"
	"github.com/muir/libschema/lspostgres"
	"github.com/muir/nject/nject"
)

func main() {
	app, err := nserve.CreateApp("myApp", users.NewUserStore, ...)
	schema := libschema.NewSchema(ctx, libschema.Options{})
	sqlDB, err := sql.Open("postgres", "....")
	database, err := lspostgres.New(logger, "main-db", schema, sqlDB)
	myhooks.MigrateMyDB.Using(database)
	err = app.Do(myhooks.MigrateMyDB)
```

### Development status

This repo represents continued development of Blue Owl's 
[nject](https://github.com/BlueOwlOpenSource/nject) base.  Blue Owl's code
has been in production use for years and has been unchanged for years.
The core of nject is mostly unchanged.  Nvelope and nserve are new.

### Go version

Due to the use of strconv.ParseComplex in nvelope, the minimum supported
version of Go is 1.15


## License
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fmuir%2Fnject.svg?type=large)](https://app.fossa.com/projects/git%2Bgithub.com%2Fmuir%2Fnject?ref=badge_large)