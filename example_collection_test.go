package nject_test

import (
	"fmt"

	"github.com/muir/nject"
)

func ExampleSequence() {
	seq := nject.Sequence("example",
		func() string {
			return "foo"
		},
		func(s string) {
			fmt.Println(s)
		},
	)
	nject.Run("run", seq)
	// Output: foo
}

func ExampleCollection_Append() {
	one := nject.Sequence("first sequence",
		func() string {
			return "foo"
		},
		func(s string) error {
			fmt.Println("from one,", s)
			// the return value means this provider isn't
			// automatically desired
			return nil
		},
	)
	two := one.Append("second sequence",
		nject.Sequence("third sequence",
			func() int {
				return 3
			},
		),
		func(s string, i int) {
			fmt.Println("from two,", s, i)
		},
	)
	fmt.Println(nject.Run("one", one))
	fmt.Println(nject.Run("two", two))
	// Output: from one, foo
	// <nil>
	// from two, foo 3
	// <nil>
}

func ExampleCollection_String() {
	one := nject.Sequence("sequence",
		func() string {
			return "foo"
		},
		func(s string) error {
			fmt.Println("from one,", s)
			// the return value means this provider isn't
			// automatically desired
			return nil
		},
	)
	fmt.Println(one)
	// Output: sequence:
	//  func() string
	//  func(string) error
}

func ExampleCollection_Condense() {
	var counter int
	one := nject.Required(nject.Sequence("one",
		func() int { counter++; return counter },
		func(b bool) string {
			return map[bool]string{
				true:  "t",
				false: "f",
			}[b]
		},
		func(s string, i int) string {
			return fmt.Sprintf("%s-%d", s, i)
		}).MustCondense(false))
	fmt.Println(nject.Run("t",
		func() bool { return true },
		one,
		func(s string) { fmt.Println(s) },
		func() bool { return false },
		one,
		func(s string) { fmt.Println(s) },
	))

	// Output: t-1
	// f-2
	// <nil>
}
