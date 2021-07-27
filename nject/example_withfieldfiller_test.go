package nject

import (
	"fmt"
)

func ExampleWithFieldFiller() {
	type S struct {
		I int `nject:"square-me"`
	}
	Run("example",
		func() int {
			return 4
		},
		MustMakeStructBuilder(&S{}, WithFieldFiller("square-me", func(i *int) {
			*i *= *i
		})),
		func(s *S) {
			fmt.Println(s.I)
		},
	)
	// Output: 16
}

func ExampleWithFieldFiller_wihtoutPointers() {
	type S struct {
		I int `nject:"square-me"`
	}
	Run("example",
		func() int {
			return 4
		},
		MustMakeStructBuilder(S{}, WithFieldFiller("square-me", func(i int) {
			fmt.Println(i * i)
		})),
		func(s S) {
			fmt.Println(s.I)
		},
	)
	// Output: 16
	// 4
}
