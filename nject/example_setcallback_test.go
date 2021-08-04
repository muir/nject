package nject_test

import (
	"fmt"

	"github.com/muir/nject/nject"
)

// SetCallback invokes a function passing a function that
// can be used to invoke a Collection
func ExampleCollection_SetCallback() {
	var cb func(string)
	fmt.Println(nject.Sequence("example",
		func() int { return 3 },
		func(s string, i int) {
			fmt.Println("got", s, i)
		},
	).SetCallback(func(f func(string)) {
		cb = f
	}))
	cb("foo")
	cb("bar")
	// Output: <nil>
	// got foo 3
	// got bar 3
}

// SetCallback invokes a function passing a function that
// can be used to invoke a Collection
func ExampleCollection_MustSetCallback() {
	var cb func(string)
	nject.Sequence("example",
		func() int { return 3 },
		func(s string, i int) {
			fmt.Println("got", s, i)
		},
	).SetCallback(func(f func(string)) {
		cb = f
	})
	cb("foo")
	cb("bar")
	// Output: got foo 3
	// got bar 3
}

// SetCallback invokes a function passing a function that
// can be used to invoke a Collection
func ExampleMustSetCallback() {
	var cb func(string)
	nject.MustSetCallback(
		nject.Sequence("example",
			func() int { return 3 },
			func(s string, i int) {
				fmt.Println("got", s, i)
			},
		), func(f func(string)) {
			cb = f
		})
	cb("foo")
	cb("bar")
	// Output: got foo 3
	// got bar 3
}
