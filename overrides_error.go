package nject

import (
	"fmt"
	"reflect"
)

// OverridesError marks a provider that is okay for that provider to override
// error returns.  Without this decorator, a wrapper that returns error but
// does not expect to receive an error will cause the injection chain
// compilation to fail.
//
// A common mistake is to have an wrapper that accidentally returns error.  It
// looks like this:
//
//	func AutoCloseThing(inner func(someType), param anotherType) error {
//		thing, err := getThing(param)
//		if err != nil {
//			return err
//		}
//		defer thing.Close()
//		inner(thing)
//		return nil
//	}
//
// The above function has two problems.  The big problem is that it will
// override any returned errors coming up from below in the call chain
// by returning nil.  The fix for this is to have the inner function return
// error.  If you aren't sure there will be something below that will
// definitely return error, then you can inject something to provide a nil
// error.  Put the following at the end of the sequence:
//
//	nject.Shun(nject.NotFinal(func () error { return nil }))
//
// The second issue is that thing.Close() probably returns error.  A correct
// wrapper for this looks like this:
//
//	func AutoCloseThing(inner func(someType) error, param anotherType) (err error) {
//		var thing someType
//		thing, err = getThing(param)
//		if err != nil {
//			return err
//		}
//		defer func() {
//			e := thing.Close()
//			if err == nil && e != nil {
//				err = e
//			}
//		}()
//		return inner(thing)
//	}
func OverridesError(fn any) Provider {
	return newThing(fn).modify(func(fm *provider) {
		fm.overridesError = true
	})
}

func checkForMissingOverridesError(collection []*provider) error {
	var errorReturnSeen bool
	for i := len(collection) - 1; i >= 0; i-- {
		fm := collection[i]
		if errorReturnSeen && !fm.overridesError && fm.class == wrapperFunc {
			consumes, returns := fm.UpFlows()
			if hasError(returns) && !hasError(consumes) {
				return fmt.Errorf("wrapper returns error but does not consume error.  Decorate with OverridesError() if this is intentional. %s", fm)
			}
		}
		if !errorReturnSeen {
			_, returns := fm.UpFlows()
			if hasError(returns) {
				errorReturnSeen = true
			}
		}
	}
	return nil
}

func hasError(types []reflect.Type) bool {
	for _, typ := range types {
		if typ == errorType {
			return true
		}
	}
	return false
}
