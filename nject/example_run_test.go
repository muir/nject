package nject

import (
	"fmt"
)

// Run is the simplest way to use the nject framework.
// Run simply executes the provider chain that it is given.
func ExampleRun() {
	providerChain := Sequence("example sequence",
		"a literal string value",
		func(s string) int {
			return len(s)
		})
	Run("example",
		providerChain,
		func(i int, s string) {
			fmt.Println(i, len(s))
		})
	// Output: 22 22
}
