package nvelope_test

import (
	"fmt"

	"github.com/muir/nject/nvelope"
)

func ExampleRecoverStack() {
	f := func(i int) (err error) {
		defer nvelope.SetErrorOnPanic(&err, nvelope.NoLogger())
		return func() error {
			switch i {
			case 0:
				panic("zero")
			case 1:
				return fmt.Errorf("a one")
			default:
				return nil
			}
		}()
	}
	err := f(0)
	fmt.Println(err)
	stack := nvelope.RecoverStack(err)
	fmt.Println(len(stack) > 1000)
	// Output: panic: zero
	// true
}
