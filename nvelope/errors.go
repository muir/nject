package nvelope

import (
	"encoding"
	"errors"
)

// ReturnCode associates an HTTP return code with a error.
// if err is nil, then nil is returned.
func ReturnCode(err error, code int) error {
	if err == nil {
		return nil
	}
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

func (err returnCode) Error() string {
	return err.cause.Error()
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

// GetReturnCode turns an error into an HTTP response code.
func GetReturnCode(err error) int {
	var rc returnCode
	if errors.As(err, &rc) {
		return rc.code
	}
	return 500
}

// CanModel represents errors that can transform themselves into a model
// for logging.
type CanModel interface {
	error
	Model() encoding.TextUnmarshaler
}
