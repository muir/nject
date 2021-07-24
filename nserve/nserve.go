package nserve

import (
	"context"
	"sync"

	"github.com/muir/nject/nject"
)

// App provides hooks to start and stop libraries that are used by an app.  It
// expected that an App corresponds to a service and that libraries that the
// service uses need to be started & stopped.
type App struct {
	lock    sync.Mutex // held when adding hooks
	runLock sync.Mutex // held when running hooks
	Hooks   map[hookId][]nject.Provider
}

// CreateApp will use nject.Run() to invoke the providers that make up the service
// represented by the app.
func CreateApp(name string, providers ...interface{}) (*App, error) {
	app := &App{
		Hooks: make(map[hookId][]nject.Provider),
	}
	ctx, cancel := context.WithCancel(context.Background())
	app.Hooks[Shutdown.Id] = append(app.Hooks[Shutdown.Id], nject.Provide("app-cancel-ctx", cancel))
	err := nject.Run(name, ctx, app, nject.Sequence("app-providers", providers...))
	return app, err
}

// On registers a callback to be invoked on hook invocation.  This can be used during
// callbacks, for example a start callback, can register a stop callback.
func (app *App) On(h *Hook, providers ...interface{}) {
	app.lock.Lock()
	defer app.lock.Unlock()
	app.Hooks[h.Id] = append(app.Hooks[h.Id], nject.Sequence("on-"+h.Name, providers...))
}

// Do invokes the callbacks for a hook.  It returns only the first error reported
// unless the hook provides an error combiner.
func (app *App) Do(h *Hook) error {
	app.runLock.Lock()
	defer app.runLock.Unlock()
	return app.do(h)
}

func (app *App) do(h *Hook) error {
	ec := h.ErrorCombiner
	if ec == nil {
		ec = func(err, _ error) error { return err }
	}
	ecw := func(e1, e2 error) error {
		if e1 == nil {
			return e2
		}
		if e2 == nil {
			return e1
		}
		return ec(e1, e2)
	}
	app.lock.Lock()
	chains := make([]nject.Provider, len(app.Hooks[h.Id]))
	copy(chains, app.Hooks[h.Id])
	app.lock.Unlock()
	var err error
	runChain := func(chain nject.Provider) {
		e := nject.Run("hook-"+h.Name, app, chain)
		err = ecw(err, e)
	}
	if h.Order == ForwardOrder {
		for _, chain := range chains {
			runChain(chain)
			if err != nil && !h.ContinuePast {
				break
			}
		}
	} else {
		for i := len(chains) - 1; i >= 0; i-- {
			chain := chains[i]
			runChain(chain)
			if err != nil && !h.ContinuePast {
				break
			}
		}
	}
	for _, oe := range h.InvokeOnError {
		err = ecw(err, app.do(oe))
	}
	return err
}
