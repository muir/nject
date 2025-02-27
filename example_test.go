package nject_test

import (
	"errors"
	"fmt"
	"strings"

	"github.com/muir/nject"
)

// Example shows what gets included and what does not for several injection chains.
// These examples are meant to show the subtlety of what gets included and why.
func Example() {
	// This demonstrates displaying the elements of a chain using an error
	// returned by the final element.
	fmt.Println(nject.Run("empty-chain",
		nject.Provide("Names", func(d *nject.Debugging) error {
			return errors.New(strings.Join(d.NamesIncluded, ", "))
		})))

	// This demonstrates that wrappers will be included if they are closest
	// provider of a return type that is required.  Names is included in
	// the upwards chain even though ReflectError could provide the error that
	// Run() wants.
	fmt.Println(nject.Run("overwrite",
		nject.Required(nject.Provide("InjectErrorDownward",
			func() error { return errors.New("overwrite me") })),
		nject.Provide("Names",
			func(inner func() error, d *nject.Debugging) error {
				_ = inner()
				return errors.New(strings.Join(d.NamesIncluded, ", "))
			}),
		nject.Provide("ReflectError",
			func(err error) error { return err })))

	// This demonstrates that the closest provider will be chosen over one farther away.
	// Otherwise InInjector would be included instead of BoolInjector and IntReinjector.
	fmt.Println(nject.Run("multiple-choices",
		nject.Provide("IntInjector", func() int { return 1 }),
		nject.Provide("BoolInjector", func() bool { return true }),
		nject.Provide("IntReinjector", func(bool) int { return 2 }),
		nject.Provide("IntConsumer", func(_ int, d *nject.Debugging) error {
			return errors.New(strings.Join(d.NamesIncluded, ", "))
		})))

	// Output: Debugging, empty-chain invoke func, Run()error, Names
	// Debugging, overwrite invoke func, Run()error, InjectErrorDownward, Names, ReflectError
	// Debugging, multiple-choices invoke func, Run()error, BoolInjector, IntReinjector, IntConsumer
}
