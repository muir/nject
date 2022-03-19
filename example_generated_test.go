package nject_test

import (
	"fmt"
	"reflect"

	"github.com/muir/nject"
)

// ExampleGeneratedFromInjectionChain demonstrates how a special
// provider can be generated that builds types that are missing
// from an injection chain.
func ExampleGenerateFromInjectionChain() {
	type S struct {
		I int
	}
	fmt.Println(nject.Run("example",
		func() int {
			return 3
		},
		nject.GenerateFromInjectionChain(
			"example",
			func(before nject.Collection, after nject.Collection) (nject.Provider, error) {
				full := before.Append("after", after)
				inputs, outputs := full.DownFlows()
				var n []interface{}
				for _, missing := range nject.ProvideRequireGap(outputs, inputs) {
					if missing.Kind() == reflect.Struct ||
						(missing.Kind() == reflect.Ptr &&
							missing.Elem().Kind() == reflect.Struct) {
						vp := reflect.New(missing)
						fmt.Println("Building filler for", missing)
						builder, err := nject.MakeStructBuilder(vp.Elem().Interface())
						if err != nil {
							return nil, err
						}
						n = append(n, builder)
					}
				}
				return nject.Sequence("build missing models", n...), nil
			}),
		func(s S, sp *S) {
			fmt.Println(s.I, sp.I)
		},
	))
	// Output: Building filler for nject_test.S
	// Building filler for *nject_test.S
	// 3 3
	// <nil>
}

func ExampleCollection_DownFlows_provider() {
	sequence := nject.Sequence("one provider", func(_ int, _ string) float64 { return 0 })
	inputs, outputs := sequence.DownFlows()
	fmt.Println("inputs", inputs)
	fmt.Println("outputs", outputs)
	// Output: inputs [int string]
	// outputs [float64]
}

func ExampleCollection_DownFlows_collection() {
	sequence := nject.Sequence("two providers",
		func(_ int, _ int64) float32 { return 0 },
		func(_ int, _ string) float64 { return 0 },
	)
	inputs, outputs := sequence.DownFlows()
	fmt.Println("inputs", inputs)
	fmt.Println("outputs", outputs)
	// Output: inputs [int int64 string]
	// outputs [float32 float64]
}

func ExampleCollection_ForEachProvider() {
	seq := nject.Sequence("example",
		func() int {
			return 10
		},
		func(_ int, _ string) {},
	)
	seq.ForEachProvider(func(p nject.Provider) {
		fmt.Println(p.DownFlows())
	})
	// Output: [] [int]
	// [int string] []
}

func ExampleCollection_Upflows() {
	var errorType = reflect.TypeOf((*error)(nil)).Elem()
	errorIsReturned := func(c nject.Provider) bool {
		_, produce := c.UpFlows()
		for _, t := range produce {
			if t == errorType {
				return true
			}
		}
		return false
	}
	collection1 := nject.Sequence("one",
		func() string {
			return "yah"
		},
		func(s string) nject.TerminalError {
			if s == "yah" {
				return fmt.Errorf("oops")
			}
			return nil
		},
		func(s string) {
			fmt.Println(s)
		},
	)
	collection2 := nject.Sequence("two",
		func() string {
			return "yah"
		},
		func(s string) {
			fmt.Println(s)
		},
	)
	collection3 := nject.Sequence("three",
		func(inner func() string) error {
			s := inner()
			if s == "foo" {
				return fmt.Errorf("not wanting foo")
			}
			return nil
		},
		func() string {
			return "foo"
		},
	)
	fmt.Println("collection1 returns error?", errorIsReturned(collection1))
	fmt.Println("collection2 returns error?", errorIsReturned(collection2))
	fmt.Println("collection3 returns error?", errorIsReturned(collection3))

	// Output: collection1 returns error? true
	// collection2 returns error? false
	// collection3 returns error? true
}
