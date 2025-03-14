package nject_test

import (
	"fmt"

	"github.com/muir/nject/v2"
)

func ExampleAllowReturnShadowing() {
	fmt.Println(nject.Run("error is shadowed",
		nject.AllowReturnShadowing[error](nject.Provide("footgun", func(inner func()) error {
			inner()
			return nil
		})),
		func() error {
			fmt.Println("error is generated")
			return fmt.Errorf("this error is dropped")
		},
	))
	// Output: error is generated
	// <nil>
}
