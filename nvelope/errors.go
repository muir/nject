package nvelope

import (
	"errors"
)

// ReturnCode associates an HTTP return code with a error.
// if err is nil, then nil is returned.
func ReturnCode(err error, code int) error {
	return returnCode{
		cause: err,
		code:  code,
	}
}

type returnCode struct {
	cause error
	code  int
}

func (err returnCode) Cause() error {
	return err.cause
}

func (err returnCode) Error() error {
	return err.Error()
}

// NotFound annotates an error has giving 404 HTTP return code
func NotFound(err error) error {
	return ReturnCode(err, 404)
}

// BadRequest annotates an error has giving 400 HTTP return code
func BadRequest(err error) error {
	return ReturnCode(err, 400)
}

// Unauthorized annotates an error has giving 401 HTTP return code
func Unauthorized(err error) error {
	return ReturnCode(err, 401)
}

// Forbidden annotates an error has giving 403 HTTP return code
func Forbidden(err error) error {
	return ReturnCode(err, 403)
}

func GetReturnCode(err error) int {
	for {
		if rc, ok := err.(returnCode); ok {
			return rc.code
		}
		if c, ok := err.(Cause); ok {
			err = c.Cause()
			continue
		}
		return 500
	}
}
