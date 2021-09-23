package nject_test

import (
	"fmt"

	"github.com/muir/nject/nject"
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
	nject.Run("example",
		func() int {
			return 4
		},
		nject.MustMakeStructBuilder(&S{},
			nject.WithMethodCall("Square"),
			nject.WithMethodCall("Print")),
		func(s *S) {
			fmt.Println("end")
		},
	)
	// Output: 16
	// end
}
