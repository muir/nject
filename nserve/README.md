# nserve - server startup/shutdown in an nject world

[![GoDoc](https://godoc.org/github.com/muir/nject/nserver?status.png)](http://godoc.org/github.com/muir/nject/nserve)

Install:

	go get github.com/muir/nject

---

This package provides server startup and shutdown wrappers that can be used
with libraries and servers that are stated with nject.

### How to structure your application

Libraries become injected dependencies.  They can in turn have other libraries
as dependencies of them.  Since only the depenencies that are required are 
actaully injected, the easiest thing is to have a master list of all your libraries
and provide that to all your apps.

Let's call that master list `allLibraries`.

	app, err := CreateApp(allLibraries, createAppFunction)
	err = app.Do(Start)
	err = app.Do(Stop)

### Hooks

Libaries and appliations can register callbacks on a per-hook basis.  Two hooks
are pre-provided by other hooks can be created.

Hook invocation can be limited by a timeout.  If the hook does not complete in
that amount of time, the hook will return error and continue processing in the
background.

The Start, Stop, and Shutdown hooks are pre-defined:

	var Shutdown NewHook(ReverseOrder)
	var Stop = NewHook(OnError(Shutdown), ReverseOrder)
	var Start = NewHook(OnError(Stop), ForwardOrder)

Hooks can be invoked in registration order (`ForwardOrder`) or in 
reverse registration order `ReverseOrder`.  

Libraries and applications can register callbacks for hooks by taking an
`*nserve.App` as as an input parameter and then using that to register callbacks:

	app.On(Start, callbackFunction)
	app.On(Stop, callbackFunction)

The callback function can be any signature.  If it is a function that can return
error, then any such error will be become the error return from `app.Do` and if
there is an `OnError` handler for that hook, that handler will be invoked.

### Context

CreateApp injects a context object into the provider list.  That context object
will be cancelled when the `Shutdown` hook is invoked.

