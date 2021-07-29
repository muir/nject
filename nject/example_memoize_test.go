package nject_test

import (
	"fmt"

	"github.com/muir/nject/nject"
)

// Memoize implies Chacheable.  To make sure that Memoize can actually function
// as desired, also mark functions with MustCache.
// With the same inputs, cached answers
// are always used.  The cache lookup examines the values passed, but does not
// do a deep insepection.
func ExampleMemoize() {
	type aStruct struct {
		ValueInStruct int
	}
	structProvider := nject.Memoize(func(ip *int, i int) *aStruct {
		return &aStruct{
			ValueInStruct: i * *ip,
		}
	})
	exampleInt := 7
	ip := &exampleInt
	_ = nject.Run("chain1",
		2,
		ip,
		structProvider,
		func(s *aStruct) {
			fmt.Println("first input", s.ValueInStruct, "value set to 22")
			s.ValueInStruct = 22
		},
	)
	_ = nject.Run("chain2",
		3,
		ip,
		structProvider,
		func(s *aStruct) {
			fmt.Println("different inputs", s.ValueInStruct)
		},
	)
	exampleInt = 33
	_ = nject.Run("chain3",
		2,
		ip,
		structProvider,
		func(s *aStruct) {
			fmt.Println("same object as first", s.ValueInStruct)
		},
	)

	// Output: first input 14 value set to 22
	// different inputs 21
	// same object as first 22
}
