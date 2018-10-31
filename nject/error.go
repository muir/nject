package nject

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
	if njerr, ok := err.(*njectError); ok {
		return njerr.err.Error() + "\n\n" + njerr.details
	}
	return err.Error()
}
