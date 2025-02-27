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

var (
	_ driver.Driver = &DummyDriver{}
	_ driver.Conn   = &DummyConn{}
	_ driver.Tx     = &DummyTx{}
)

type DummyDriver struct{}

func (*DummyDriver) Open(_ string) (driver.Conn, error) {
	fmt.Println("db open")
	return &DummyConn{}, nil
}

type DummyConn struct{}

func (*DummyConn) Prepare(_ string) (driver.Stmt, error) {
	return nil, fmt.Errorf("not implemented")
}

func (*DummyConn) Close() error {
	fmt.Println("db close")
	return nil
}

func (*DummyConn) Begin() (driver.Tx, error) {
	fmt.Println("tx begin")
	return &DummyTx{}, nil
}

type DummyTx struct{}

func (*DummyTx) Commit() error {
	fmt.Println("tx committed")
	return nil
}

func (*DummyTx) Rollback() error {
	fmt.Println("tx rolled back")
	return nil
}
