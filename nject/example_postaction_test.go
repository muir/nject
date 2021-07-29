package nject_test

import (
	"fmt"

	"github.com/muir/nject/nject"
)

func ExamplePostActionByTag() {
	type S struct {
		I int `nject:"square-me"`
	}
	nject.Run("example",
		func() int {
			return 4
		},
		nject.MustMakeStructBuilder(&S{}, nject.PostActionByTag("square-me", func(i *int) {
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
	nject.Run("example",
		func() int {
			return 4
		},
		nject.MustMakeStructBuilder(S{}, nject.PostActionByTag("square-me", func(i int) {
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
	fmt.Println(nject.Run("example",
		func() int32 {
			return 10
		},
		func() *[]int {
			var x []int
			return &x
		},
		nject.MustMakeStructBuilder(S{},
			nject.PostActionByTag("rollup", func(i int, a *[]int) {
				*a = append(*a, i+1)
			}),
			nject.PostActionByTag("rolldown", func(i int64, a *[]int) {
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
	fmt.Println(nject.Run("example",
		func() int32 {
			return 10
		},
		func() *[]int {
			var x []int
			return &x
		},
		nject.MustMakeStructBuilder(S{},
			nject.PostActionByName("I", func(i int, a *[]int) {
				*a = append(*a, i+1)
			}),
			nject.PostActionByName("J", func(i int64, a *[]int) {
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

func ExamplePostActionByType() {
	type S struct {
		I int32
		J int64
	}
	fmt.Println(nject.Run("example",
		func() int32 {
			return 10
		},
		func() int64 {
			return 20
		},
		func() *[]int {
			var x []int
			return &x
		},
		nject.MustMakeStructBuilder(&S{},
			nject.PostActionByType(func(i int32, a *[]int) {
				*a = append(*a, int(i))
			}),
			nject.PostActionByType(func(i *int32, a *[]int) {
				*i += 5
			}),
		),
		func(s *S, a *[]int) {
			fmt.Println(*a, s.I, s.J)
		},
	))
	// Output: [15] 15 20
	// <nil>
}
