package nject_test

import (
	"fmt"

	"github.com/muir/nject"
)

type Causer interface {
	Unwrap() error
	Error() string
}

type MyError struct {
	err error
}

func (err MyError) Error() string {
	return "MY: " + err.err.Error()
}

func (err MyError) Unwrap() error {
	return err.err
}

var _ Causer = MyError{}

func ExamplePostActionByTag_withInterfaces() {
	type S struct {
		Error Causer `nject:"print-error,print-cause"`
	}
	fmt.Println(nject.Run("example",
		func() error {
			return fmt.Errorf("an injected error")
		},
		func(err error) Causer {
			return MyError{err: err}
		},
		nject.MustMakeStructBuilder(S{},
			nject.PostActionByTag("print-error", func(err error) {
				fmt.Println(err)
			}),
			nject.PostActionByTag("print-cause", func(err Causer) {
				fmt.Println("Cause:", err.Unwrap())
			}),
		),
		func(_ S) {
			fmt.Println("Done")
		},
	))
	// Output: MY: an injected error
	// Cause: an injected error
	// Done
	// <nil>
}
