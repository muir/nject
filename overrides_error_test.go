package nject

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOverridesError(t *testing.T) {
	type someType string
	type anotherType string
	getThing := func(_ anotherType) (someType, error) { return "", nil }
	danger := Required(func(inner func(someType), param anotherType) error {
		thing, err := getThing(param)
		if err != nil {
			return err
		}
		inner(thing)
		return nil
	})
	finalWithError := func() error { return nil }
	returnsTerminal := Required(func() TerminalError { return nil })
	finalWithoutError := func() {}
	var target func(anotherType) error

	t.Log("test: okay because no error bubbling up")
	//nolint:testifylint // assert is okay
	assert.NoError(t, Sequence("A", danger, finalWithoutError).Bind(&target, nil))

	t.Log("test: should fail because the final function returns error that gets clobbered")
	//nolint:testifylint // assert is okay
	assert.Error(t, Sequence("B", danger, finalWithError).Bind(&target, nil))

	t.Log("test: should fail because there is a terminal-error injector that gets clobbered")
	//nolint:testifylint // assert is okay
	assert.Error(t, Sequence("C", danger, returnsTerminal, finalWithoutError).Bind(&target, nil))

	t.Log("test: okay because marked even though the final function returns error that gets clobbered")
	//nolint:testifylint // assert is okay
	assert.NoError(t, Sequence("B", OverridesError(danger), finalWithError).Bind(&target, nil))

	t.Log("test: okay because marked even though there is a terminal-error injector that gets clobbered")
	assert.NoError(t, Sequence("C", OverridesError(danger), returnsTerminal, finalWithoutError).Bind(&target, nil))
}
