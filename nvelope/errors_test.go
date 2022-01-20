package nvelope_test

import (
	"fmt"
	"testing"

	"github.com/muir/nject/nvelope"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestErrors(t *testing.T) {
	assert.Equal(t, 304, nvelope.GetReturnCode(nvelope.ReturnCode(fmt.Errorf("x"), 304)), "unwrapped")
	assert.Equal(t, 303, nvelope.GetReturnCode(errors.Wrap(nvelope.ReturnCode(fmt.Errorf("x"), 303), "o")), "wrapped")
	assert.Equal(t, 400, nvelope.GetReturnCode(nvelope.BadRequest(fmt.Errorf("x"))), "bad")
	assert.Equal(t, 401, nvelope.GetReturnCode(nvelope.Unauthorized(fmt.Errorf("x"))), "unauth")
	assert.Equal(t, 403, nvelope.GetReturnCode(nvelope.Forbidden(fmt.Errorf("x"))), "forbid")
}
