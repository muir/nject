package nject_test

import (
	"testing"

	"github.com/muir/nject"
	"github.com/stretchr/testify/require"
)

func TestShadowingAnnotation(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		valid   bool
		wrapper nject.Provider
	}{
		{
			name:  "noGun",
			valid: true,
			wrapper: nject.Provide("nogun", func(inner func() error) error {
				return inner()
			}),
		},
		{
			name:  "footgun",
			valid: false,
			wrapper: nject.Provide("gun", func(inner func()) error {
				inner()
				return nil
			}),
		},
		{
			name:  "forced",
			valid: true,
			wrapper: nject.AllowReturnShadowing[error](nject.Provide("gun", func(inner func()) error {
				inner()
				return nil
			})),
		},
		{
			name:  "wrong",
			valid: false,
			wrapper: nject.AllowReturnShadowing[string](nject.Provide("gun", func(inner func()) error {
				inner()
				return nil
			})),
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			c := nject.Sequence(tc.name,
				tc.wrapper,
				func() error {
					return nil
				},
			)
			var invoke func() error
			err := c.Bind(&invoke, nil)
			if tc.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				t.Logf("error is %s", err.Error())
			}
		})
	}
}
