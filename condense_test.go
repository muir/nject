package nject_test

import (
	"fmt"
	"testing"

	"github.com/muir/nject"

	"github.com/stretchr/testify/assert"
)

func TestCondense(t *testing.T) {
	t.Parallel()

	var x func(int) error
	nject.Sequence("s1",
		nject.Sequence("s",
			func(i int) nject.TerminalError {
				if i == 1 {
					return fmt.Errorf("1")
				}
				return nil
			},
			func(i int) error {
				if i == 2 {
					return fmt.Errorf("2")
				}
				return nil
			}).MustCondense(true),
		func(i int) nject.TerminalError {
			if i == 3 {
				return fmt.Errorf("3")
			}
			return nil
		},
		func(i int) error {
			if i == 4 {
				return fmt.Errorf("4")
			}
			return nil
		}).Bind(&x, nil)
	c := func(i int) string {
		err := x(i)
		if err == nil {
			return ""
		}
		return err.Error()
	}
	assert.Equal(t, "1", c(1))
	assert.Equal(t, "2", c(2))
	assert.Equal(t, "3", c(3))
	assert.Equal(t, "4", c(4))
	assert.Equal(t, "", c(0))
}
