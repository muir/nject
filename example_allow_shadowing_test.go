package nject_test

import (
	"fmt"

	"github.com/muir/nject"
)

func ExampleAllowReturnShadowing() {
	someCondition := false
	fmt.Println(nject.Run("error is shadowed",
		nject.AllowReturnShadowing[error](nject.Provide("footgun", func(inner func()) error {
			if someCondition {
				return fmt.Errorf("some condition happened")
			}
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
