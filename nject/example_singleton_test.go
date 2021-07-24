package nject

import (
	"fmt"
)

// Singleton providers get run only once even if their arguments are different.
func ExampleSingleton() {
	type aStruct struct {
		ValueInStruct int
	}
	structProvider := Singleton(func(s string, i int) *aStruct {
		return &aStruct{
			ValueInStruct: len(s) * i,
		}
	})
	_ = Run("chain1",
		"four",
		4,
		structProvider,
		func(a *aStruct, s string, i int) {
			fmt.Printf("inputs are %s and %d, value is %d\n", s, i, a.ValueInStruct)
		},
	)
	_ = Run("chain2",
		"seven",
		5,
		structProvider,
		func(a *aStruct, s string, i int) {
			fmt.Printf("inputs are %s and %d, value is %d\n", s, i, a.ValueInStruct)
		},
	)

	// Output: inputs are four and 4, value is 16
	// inputs are seven and 5, value is 16
}
