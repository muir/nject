package nserve_test

import (
	"fmt"

	"github.com/muir/nject/nserve"
	"github.com/pkg/errors"
)

type (
	L1 struct{}
	L2 struct{}
	L3 struct{}
)

func NewL1(app *nserve.App) *L1 {
	fmt.Println("L1 created")
	app.On(nserve.Start, func(app *nserve.App) {
		app.On(nserve.Stop, func() error {
			fmt.Println("L1 stopped")
			return fmt.Errorf("L1 stop error")
		})
		fmt.Println("L1 started")
	})
	return &L1{}
}

func NewL2(app *nserve.App, _ *L1) *L2 {
	fmt.Println("L2 created")
	app.On(nserve.Start, func(app *nserve.App) error {
		app.On(nserve.Stop, func() error {
			fmt.Println("L2 stopped")
			return fmt.Errorf("L2 stop error")
		})
		fmt.Println("L2 started")
		// Note: Library2 start will return error
		return fmt.Errorf("L2 start error")
	})
	return &L2{}
}

func NewL3(_ *L2, app *nserve.App) *L3 {
	fmt.Println("L3 created")
	app.On(nserve.Start, func(app *nserve.App) {
		fmt.Println("L3 started")
	})
	return &L3{}
}

func ErrorCombiner(e1, e2 error) error {
	return errors.New(e1.Error() + "; " + e2.Error())
}

// Example shows the injection, startup, and shutdown of an app with two libraries
func Example() {
	nserve.Start.SetErrorCombiner(ErrorCombiner)
	nserve.Stop.SetErrorCombiner(ErrorCombiner)
	nserve.Shutdown.SetErrorCombiner(ErrorCombiner)
	app, err := nserve.CreateApp("myApp", NewL1, NewL2, NewL3, func(_ *L1, _ *L2, _ *L3, app *nserve.App) {
		fmt.Println("App created")
	})
	fmt.Println("create error:", err)
	err = app.Do(nserve.Start)
	fmt.Println("do start error:", err)
	// Output: L1 created
	// L2 created
	// L3 created
	// App created
	// create error: <nil>
	// L1 started
	// L2 started
	// L1 stopped
	// L2 stopped
	// do start error: L2 start error; L1 stop error; L2 stop error
}
