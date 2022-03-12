package nject_test

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
)

// some DummyDriver parts taken from
// https://github.com/vyskocilm/gazpacho/tree/master/dbmagic
func init() {
	sql.Register("dummy", &DummyDriver{})
}

var _ driver.Driver = &DummyDriver{}
var _ driver.Conn = &DummyConn{}
var _ driver.Tx = &DummyTx{}

type DummyDriver struct{}

func (d *DummyDriver) Open(name string) (driver.Conn, error) {
	fmt.Println("db open")
	return &DummyConn{}, nil
}

type DummyConn struct{}

func (c *DummyConn) Prepare(query string) (driver.Stmt, error) {
	return nil, fmt.Errorf("not implemented")
}

func (c *DummyConn) Close() error {
	fmt.Println("db close")
	return nil
}

func (c *DummyConn) Begin() (driver.Tx, error) {
	fmt.Println("tx begin")
	return &DummyTx{}, nil
}

type DummyTx struct{}

func (t *DummyTx) Commit() error {
	fmt.Println("tx committed")
	return nil
}

func (t *DummyTx) Rollback() error {
	fmt.Println("tx rolled back")
	return nil
}
