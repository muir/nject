package nject_test

import (
	"testing"

	"github.com/muir/nject"
	"github.com/stretchr/testify/assert"
)

type PT01 string
type PT02 string
type PT03 string
type PT04 string

func TestParallelCallsToInner(t *testing.T) {
	t.Parallel()
	assert.NoError(t, nject.Run(t.Name(),
		t,
		nject.Parallel(func(inner func(*testing.T, PT01), t *testing.T) {
			for _, s := range []PT01{"A1", "A2", "A3", "A4"} {
				s := s
				t.Run(string(s), func(t *testing.T) {
					t.Log("branching")
					t.Parallel()
					inner(t, s)
				})
			}
		}),
		nject.Parallel(func(inner func(*testing.T, PT02), t *testing.T) {
			for _, s := range []PT02{"B1", "B2", "B3", "B4"} {
				s := s
				t.Run(string(s), func(t *testing.T) {
					t.Log("branching")
					t.Parallel()
					inner(t, s)
				})
			}
		}),
		nject.Parallel(func(inner func(*testing.T, PT03), t *testing.T) {
			for _, s := range []PT03{"C1", "C2", "C3", "C4"} {
				s := s
				t.Run(string(s), func(t *testing.T) {
					t.Log("branching")
					t.Parallel()
					inner(t, s)
				})
			}
		}),
		nject.Parallel(func(inner func(*testing.T, PT04), t *testing.T) {
			for _, s := range []PT04{"D1", "D2", "D3", "D4"} {
				s := s
				t.Run(string(s), func(t *testing.T) {
					t.Log("branching")
					t.Parallel()
					inner(t, s)
				})
			}
		}),
		func(t *testing.T, a PT01, b PT02, c PT03, d PT04) {
			assert.Equal(t, t.Name(), "TestParallelCallsToInner/"+string(a)+"/"+string(b)+"/"+string(c)+"/"+string(d))
		},
	))
}
