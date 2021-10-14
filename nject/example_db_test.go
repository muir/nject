package nject_test

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/muir/nject/nject"
)

// InjectDB injects both a *sql.DB and *sql.Tx if they're needed.
// Errors from opening and closing the database can be returned
// so a consumer of downstream errors is necessary.
// A context.Context is used in the creation of the transaction
// inject that earlier in the chain.  txOptions can be nil.
func InjectDB(driver, uri string, txOptions *sql.TxOptions) *nject.Collection {
	return nject.Sequence("database-sequence",
		driverType(driver),
		uriType(uri),
		txOptions,

		// We tag the db injector as MustConsume so that we don't inject
		// the database unless the is a consumer for it.  When a wrapper
		// returns error, it should usually consume error too and pass
		// that error along, otherwise it can mask a downstream error.
		nject.MustConsume(nject.Provide("db", injectDB)),

		// We tag the tx injector as MustConsume so that we don't inject
		// the transaction unless the is a consumer for it.  When a wrapper
		// returns error, it should usually consume error too and pass
		// that error along, otherwise it can mask a downstream error.
		nject.MustConsume(nject.Provide("tx", injectTx)),

		// Since injectTx or injectDB consumes an error, this provider
		// will supply that error if there is no other downstream supplier
		nject.Shun(nject.Provide("fallback error", fallbackErrorSource)),
	)
}

type driverType string
type uriType string

func injectDB(inner func(*sql.DB) error, driver driverType, uri uriType) (finalError error) {
	db, err := sql.Open(string(driver), string(uri))
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

func injectTx(inner func(*sql.Tx) error, ctx context.Context, db *sql.DB, opts *sql.TxOptions) (finalError error) {
	tx, err := db.BeginTx(ctx, opts)
	if err != nil {
		return err
	}
	defer func() {
		if finalError == nil {
			finalError = tx.Commit()
			if finalError == sql.ErrTxDone {
				finalError = nil
			}
		} else {
			_ = tx.Rollback()
		}
	}()
	return inner(tx)
}

// This has to be nject.TerminalError instead of error so that
// it gets consumed upstream instead of downstream
func fallbackErrorSource() nject.TerminalError {
	fmt.Println("fallback error returns nil")
	return nil
}

// This example explores injecting a database
// handle or transaction only when they're used.
func Example_transaction() {
	// InjectDB will want a context and will return an error
	upstream := func(inner func(context.Context) error) {
		err := inner(context.Background())
		if err != nil {
			fmt.Println(err.Error())
		}
	}

	fmt.Println("No database used...")
	nject.MustRun("A", upstream, InjectDB("dummy", "ignored", nil),
		func() {
			fmt.Println("final-func")
		})

	fmt.Println("\nDatabase used...")
	nject.MustRun("B", upstream, InjectDB("dummy", "ignored", nil),
		func(db *sql.DB) error {
			_, _ = db.Prepare("ignored") // database opens are lazy so this triggers the logging
			fmt.Println("final-func")
			return nil
		})

	fmt.Println("\nTransaction used...")
	nject.MustRun("C", upstream, InjectDB("dummy", "ignored", nil),
		func(_ *sql.Tx) {
			fmt.Println("final-func")
		})

	// Output: No database used...
	// final-func
	//
	// Database used...
	// db open
	// final-func
	// db close
	//
	// Transaction used...
	// db open
	// tx begin
	// fallback error returns nil
	// final-func
	// tx committed
	// db close
}
