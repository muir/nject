package nject_test

import (
	"fmt"

	"github.com/muir/nject/v2"
)

// This demonstrates how it to have a default that gets overridden by
// by later inputs using Reorder
func ExampleReorder() {
	type string2 string
	seq1 := nject.Sequence("example",
		nject.Shun(func() string {
			fmt.Println("fallback default included")
			return "fallback default"
		}),
		func(s string) string2 {
			return "<" + string2(s) + ">"
		},
	)
	seq2 := nject.Sequence("later inputs",
		// for this to work, it must be reordered to be in front
		// of the string->string2 provider
		nject.Reorder(func() string {
			return "override value"
		}),
	)
	fmt.Println(nject.Run("combination",
		seq1,
		seq2,
		func(s string2) {
			fmt.Println(s)
		},
	))
	// Output: <override value>
	// <nil>
}

// This demonstrates how it to have a default that gets overridden by
// by later inputs using ReplaceNamed
func ExampleReplaceNamed() {
	type string2 string
	seq1 := nject.Sequence("example",
		nject.Provide("default-string", func() string {
			fmt.Println("fallback default included")
			return "fallback default"
		}),
		func(s string) string2 {
			return "<" + string2(s) + ">"
		},
	)
	seq2 := nject.Sequence("later inputs",
		nject.ReplaceNamed("default-string", func() string {
			return "override value"
		}),
	)
	fmt.Println(nject.Run("combination",
		seq1,
		seq2,
		func(s string2) {
			fmt.Println(s)
		},
	))
	// Output: <override value>
	// <nil>
}
