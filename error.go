package nject

import (
	"errors"
	"sync"
)

type njectError struct {
	err     error
	details string
}

func (ne *njectError) Error() string {
	return ne.err.Error()
}

// DetailedError transforms errors into strings.  If
// the error happens to be an error returned by Bind()
// or something that called Bind() then it will return
// a much more detailed error than just calling err.Error()
func DetailedError(err error) string {
	if err == nil {
		return ""
	}
	var njectError *njectError
	if errors.As(err, &njectError) {
		dups := duplicateTypes()
		if dups != "" {
			return err.Error() + "\n\n" + njectError.details +
				"\n\nWarning: the following type names refer to more than one type:\n" +
				dups
		}
		return err.Error() + "\n\n" + njectError.details
	}
	return err.Error()
}

var (
	duplicatesThrough int
	dupLock           sync.Mutex
	duplicates        string
	duplicatesFound   = make(map[string]struct{})
)

func duplicateTypes() string {
	maxTC := func() int {
		lock.Lock()
		defer lock.Unlock()
		return typeCounter
	}()
	dupLock.Lock()
	defer dupLock.Unlock()
	if duplicatesThrough == maxTC {
		return duplicates
	}
	names := make(map[string]struct{})
	for i := 1; i <= maxTC; i++ {
		n := typeCode(i).String()
		if _, ok := names[n]; ok {
			if _, ok := duplicatesFound[n]; !ok {
				duplicates += " " + n
				duplicatesFound[n] = struct{}{}
			}
		}
		names[n] = struct{}{}
	}
	duplicatesThrough = maxTC
	return duplicates
}
