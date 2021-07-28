package nject

import (
	"fmt"
)

type S struct {
	I int
}

func (s *S) Square() {
	s.I *= s.I
}

func (s S) Print() {
	fmt.Println(s.I)
}

func ExampleMethodCall() {
	Run("example",
		func() int {
			return 4
		},
		MustMakeStructBuilder(&S{}, WithMethodCall("Square"), WithMethodCall("Print")),
		func(s *S) {
			fmt.Println("end")
		},
	)
	// Output: 16
	// end
}
