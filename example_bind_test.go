package nject_test

import (
	"fmt"

	"github.com/muir/nject"
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
	err := providerChain.Bind(&aInvoke, &aInit)
	if err != nil {
		panic(err)
	}
	aInit("string comes from init")
	aInit("ignored since invoke is done")
	aInvoke()
	aInvoke()

	var bInvoke func(string)
	err = providerChain.Bind(&bInvoke, nil)
	if err != nil {
		panic(err)
	}
	bInvoke("string comes from invoke")
	bInvoke("not a constant")

	// Output: string comes from init 22
	// string comes from init 22
	// string comes from invoke 24
	// not a constant 14
}

// Parameters can be passed to both the init and then
// invoke functions when using Bind.
func ExampleCollection_Bind_passing_in_parameters() {
	chain := nject.Sequence("example",
		nject.Provide("static-injector",
			// This will be a static injector because its input
			// will come from the bind init function
			func(s string) int {
				return len(s)
			}),
		nject.Provide("regular-injector",
			// This will be a regular injector because its input
			// will come from the bind invoke function
			func(i int32) int64 {
				return int64(i)
			}),
		nject.Provide("final-injector",
			// This will be the last injector in the chain and thus
			// is the final injector and it must be included
			func(i int64, j int) int64 {
				fmt.Println(i, j)
				return i + int64(j)
			}),
	)
	var initFunc func(string)
	var invokeFunc func(int32) int64
	fmt.Println(chain.Bind(&invokeFunc, &initFunc))
	initFunc("example thirty-seven character string")
	fmt.Println(invokeFunc(10))
	// Output: <nil>
	// 10 37
	// 47
}

func ExampleMustBindSimple() {
	f := nject.MustBindSimple(
		nject.Sequence("example",
			func() int {
				return 7
			},
			func(i int) {
				fmt.Println(i)
			},
		), "bind-name")
	f()
	f()
	// Output: 7
	// 7
}

func ExampleCollection_MustBindSimple() {
	f := nject.Sequence("example",
		func() int {
			return 7
		},
		func(i int) {
			fmt.Println(i)
		},
	).MustBindSimple()
	f()
	f()
	// Output: 7
	// 7
}

func ExampleMustBindSimpleError() {
	f := nject.MustBindSimpleError(
		nject.Sequence("example",
			func() int {
				return 7
			},
			func(i int) error {
				fmt.Println(i)
				return nil
			},
		), "bind-name")
	fmt.Println(f())
	// Output: 7
	// <nil>
}

func ExampleCollection_MustBindSimpleError() {
	f := nject.Sequence("example",
		func() int {
			return 7
		},
		func(i int) error {
			fmt.Println(i)
			return nil
		},
	).MustBindSimpleError()
	fmt.Println(f())
	// Output: 7
	// <nil>
}
