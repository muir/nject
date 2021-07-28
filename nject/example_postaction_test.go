package nject

import (
	"fmt"
)

func ExamplePostActionByTag() {
	type S struct {
		I int `nject:"square-me"`
	}
	Run("example",
		func() int {
			return 4
		},
		MustMakeStructBuilder(&S{}, PostActionByTag("square-me", func(i *int) {
			*i *= *i
		})),
		func(s *S) {
			fmt.Println(s.I)
		},
	)
	// Output: 16
}

func ExamplePostActionByTag_wihtoutPointers() {
	type S struct {
		I int `nject:"square-me"`
	}
	Run("example",
		func() int {
			return 4
		},
		MustMakeStructBuilder(S{}, PostActionByTag("square-me", func(i int) {
			fmt.Println(i * i)
		})),
		func(s S) {
			fmt.Println(s.I)
		},
	)
	// Output: 16
	// 4
}

func ExamplePostActionByTag_conversion() {
	type S struct {
		I int32 `nject:"rollup"`
		J int32 `nject:"rolldown"`
	}
	fmt.Println(Run("example",
		func() int32 {
			return 10
		},
		func() *[]int {
			var x []int
			return &x
		},
		MustMakeStructBuilder(S{},
			PostActionByTag("rollup", func(i int, a *[]int) {
				*a = append(*a, i+1)
			}),
			PostActionByTag("rolldown", func(i int64, a *[]int) {
				*a = append(*a, int(i)-1)
			}),
		),
		func(_ S, a *[]int) {
			fmt.Println(*a)
		},
	))
	// Output: [11 9]
	// <nil>
}

func ExamplePostActionByName() {
	type S struct {
		I int32
		J int32
	}
	fmt.Println(Run("example",
		func() int32 {
			return 10
		},
		func() *[]int {
			var x []int
			return &x
		},
		MustMakeStructBuilder(S{},
			PostActionByName("I", func(i int, a *[]int) {
				*a = append(*a, i+1)
			}),
			PostActionByName("J", func(i int64, a *[]int) {
				*a = append(*a, int(i)-1)
			}),
		),
		func(_ S, a *[]int) {
			fmt.Println(*a)
		},
	))
	// Output: [11 9]
	// <nil>
}
