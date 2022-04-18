package nject_test

import (
	"fmt"
	"strconv"

	"github.com/muir/nject"
)

// Provide does one job: it names an otherwise anonymous
// function so that it easier to identify if there is an
// error creating an injection chain.
func ExampleProvide() {
	fmt.Println(nject.Run("failure1",
		func(s string) int {
			return 4
		},
	))
	fmt.Println(nject.Run("failure2",
		nject.Provide("create-int", func(s string) int {
			return 4
		}),
	))
	// Output: final-func: failure1(0) [func(string) int]: required but has no match for its input parameter string
	// final-func: create-int [func(string) int]: required but has no match for its input parameter string
}

func ExampleProvide_literal() {
	fmt.Println(nject.Run("literals",
		nject.Provide("an int", 7),
		"I am a literal string", // naked literal work too
		nject.Provide("I-am-a-final-func", func(s string, i int) {
			fmt.Println("final:", s, i)
		}),
	))
	// Output: final: I am a literal string 7
	// <nil>
}

func ExampleProvide_regular_injector() {
	fmt.Println(nject.Run("regular",
		func() int {
			return 7
		},
		nject.Provide("convert-int-to-string",
			func(i int) string {
				return strconv.Itoa(i)
			},
		),
		func(s string) {
			fmt.Println(s)
		},
	))
	// Output: 7
	// <nil>
}

// This demonstrates multiple types of injectors including a
// wrapper and a fallible injector
func ExampleProvide_wrapper_and_fallible_injectors() {
	shouldFail := true
	seq := nject.Sequence("fallible",
		nject.Provide("example-wrapper",
			func(inner func() (string, error)) {
				s, err := inner()
				fmt.Println("string:", s, "error:", err)
			}),
		nject.Provide("example-injector",
			func() bool {
				return shouldFail
			}),
		nject.Provide("example-fallible-injector",
			func(b bool) (string, nject.TerminalError) {
				if b {
					return "", fmt.Errorf("oops, failing")
				}
				return "example", nil
			}),
		nject.Provide("example-final-injector",
			func(s string) string {
				return "final: " + s
			}),
	)
	fmt.Println(nject.Run("failure", seq))
	shouldFail = false
	fmt.Println(nject.Run("success", seq))
	// Output: string:  error: oops, failing
	// oops, failing
	// string: final: example error: <nil>
	// <nil>
}

// This demonstrate the use of NonFinal.  NonFinal is useful when
// manipulating lists of providers.
func ExampleNonFinal() {
	seq := nject.Sequence("example",
		func() string {
			return "some string"
		},
		func(i int, s string) {
			fmt.Println("final", i, s)
		},
	)
	fmt.Println(nject.Run("almost incomplete",
		seq,
		nject.NonFinal(func() int {
			return 20
		}),
	))
	// Output: final 20 some string
	// <nil>
}

// This demonstrates how it to have a default that gets overridden by
// by later inputs.
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
