package nject_test

import (
	"database/sql"

	"github.com/muir/nject"
)

type (
	driverName     string
	dataSourceName string
)

// openDBErrorReturnRequired is a provider that opens a database. On the surface it seems
// fine but it has a problem: what if nothing below it returns error?
//
//nolint:unused // function is just an example
func openDBErrorReturnRequired(inner func(*sql.DB) error, driver driverName, name dataSourceName) error {
	db, err := sql.Open(string(driver), string(name))
	if err != nil {
		return err
	}
	defer db.Close()
	return inner(db)
}

// openDBCollection is a collection of providers that open a database but do not
// assume that a something farther down the chain will return error.  Since this collection
// may be used nievely in a context where someone is trying to cache things,
// NotCacheable is used to make sure that we do not cache the open.
// We use MustConsume and a private type on the open to make sure that if the open happens,
// the close will happen too.
type mustCloseDB bool // private type
var openDBCollection = nject.Sequence("open-database",
	nject.NotCacheable(nject.MustConsume[mustCloseDB](nject.MustConsume[*sql.DB](
		func(driver driverName, name dataSourceName) (*sql.DB, mustCloseDB, nject.TerminalError) {
			db, err := sql.Open(string(driver), string(name))
			if err != nil {
				return nil, false, err
			}
			return db, false, nil
		}))),
	func(inner func(*sql.DB), db *sql.DB, _ mustCloseDB) {
		defer db.Close()
		inner(db)
	},
)

// ExampleNotCacheable is a function to demonstrate the use of NotCacheable and
// MustConsume.
func ExampleNotCacheable() {
	// If someone tries to make things faster by marking everything as Cacheable,
	// the NotCacheable in openDBCollection() will prevent an inappropriate move to the
	// static chain of the database open.
	_ = nject.Cacheable(nject.Sequence("big collection",
		// Many providers
		driverName("postgres"),
		dataSourceName("postgresql://username:password@host:port/databasename"),
		openDBCollection,
		// Many other providers here
	))
}
