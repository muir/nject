package nject

import (
	"database/sql"
)

type DriverName string
type DataSourceName string

// OpenDBErrorReturnRequired is a provider that opens a database.   Surface it seems
// fine but it has a problem: what if nothing below it returns error?
func OpenDBErrorReturnRequired(inner func(*sql.DB) error, driver DriverName, name DataSourceName) error {
	db, err := sql.Open(string(driver), string(name))
	if err != nil {
		return err
	}
	defer db.Close()
	return inner(db)
}

// OpenDBCollection is a collection of providers that open a database but do not
// assume that a something farther down the chain will return error.  Since this collection
// may be used nievely in a context where someone is trying to cache things,
// NotCacheable is used to make sure that we do not cache the open.
var OpenDBCollection = Sequence("open-database",
	NotCacheable(func(driver DriverName, name DataSourceName) (*sql.DB, TerminalError) {
		db, err := sql.Open(string(driver), string(name))
		if err != nil {
			return nil, err
		}
		return db, nil
	}),
	func(inner func(*sql.DB), db *sql.DB) {
		defer db.Close()
		inner(db)
	},
)

func ExampleNotCacheable() {
	// If someone tries to make things faster by marking everything as Cacheable,
	// the NotCacheable in OpenDBCollection() will prevent an inappropriate move to the
	// static chain of the database open.
	_ = Cacheable(Sequence("big collection",
		// Many providers
		DriverName("postgres"),
		DataSourceName("postgresql://username:password@host:port/databasename"),
		OpenDBCollection,
		// Many other providers here
	))
}
