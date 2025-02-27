package nject_test

import (
	"fmt"

	"github.com/muir/nject"
)

type S struct {
	I int
}

func (s *S) Square() {
	s.I *= s.I
}

func (s *S) Print() {
	fmt.Println(s.I)
}

func ExampleWithMethodCall() {
	nject.MustRun("example",
		func() int {
			return 4
		},
		nject.MustMakeStructBuilder(&S{},
			nject.WithMethodCall("Square"),
			nject.WithMethodCall("Print")),
		func(_ *S) {
			fmt.Println("end")
		},
	)
	// Output: 16
	// end
}
