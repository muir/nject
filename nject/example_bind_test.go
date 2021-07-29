package nject_test

import (
	"fmt"

	"github.com/muir/nject/nject"
)

// Bind does as much work before invoke as possible.
func ExampleCollection_Bind() {
	providerChain := nject.Sequence("example sequence",
		func(s string) int {
			return len(s)
		},
		func(i int, s string) {
			fmt.Println(s, i)
		})

	var aInit func(string)
	var aInvoke func()
	providerChain.Bind(&aInvoke, &aInit)
	aInit("string comes from init")
	aInit("ignored since invoke is done")
	aInvoke()
	aInvoke()

	var bInvoke func(string)
	providerChain.Bind(&bInvoke, nil)
	bInvoke("string comes from invoke")
	bInvoke("not a constant")

	// Output: string comes from init 22
	// string comes from init 22
	// string comes from invoke 24
	// not a constant 14
}
