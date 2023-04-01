package nject_test

import (
	"strconv"
	"testing"

	"github.com/muir/nject"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReplaceSingles(t *testing.T) {
	t.Parallel()
	type action struct {
		at     int
		target int
		op     func(target string, fn interface{}) nject.Provider
	}
	cases := []struct {
		name    string
		n       int
		actions []action
		want    string
	}{
		{
			name: "no replacments",
			n:    10,
			want: "> 1 2 3 4 4 5 6 6 6 7 8 8 9 9 9 10 10 10 10 10",
		},
		{
			name: "one replace back, middle",
			n:    10,
			want: "> 1 2 7 4 4 5 6 6 6 8 8 9 9 9 10 10 10 10 10",
			actions: []action{
				{
					at:     7,
					target: 3,
					op:     nject.ReplaceNamed,
				},
			},
		},
		{
			name: "one replace forward, middle",
			n:    10,
			want: "> 1 2 4 4 5 6 6 6 3 8 8 9 9 9 10 10 10 10 10",
			actions: []action{
				{
					at:     3,
					target: 7,
					op:     nject.ReplaceNamed,
				},
			},
		},
		{
			name: "replace one with one forward, at end",
			n:    11,
			want: "> 1 2 4 4 5 6 6 6 7 8 8 9 9 9 10 10 10 10 10 3",
			actions: []action{
				{
					at:     3,
					target: 11,
					op:     nject.ReplaceNamed,
				},
			},
		},
		{
			name: "replace mult with one back, middle",
			n:    10,
			want: "> 1 2 3 7 5 6 6 6 8 8 9 9 9 10 10 10 10 10",
			actions: []action{
				{
					at:     7,
					target: 4,
					op:     nject.ReplaceNamed,
				},
			},
		},
		{
			name: "replace mult with one forward, middle",
			n:    10,
			want: "> 1 3 4 4 5 6 6 6 7 8 8 2 10 10 10 10 10",
			actions: []action{
				{
					at:     2,
					target: 9,
					op:     nject.ReplaceNamed,
				},
			},
		},
		{
			name: "replace mult with one forward, at end",
			n:    10,
			want: "> 1 3 4 4 5 6 6 6 7 8 8 9 9 9 2",
			actions: []action{
				{
					at:     2,
					target: 10,
					op:     nject.ReplaceNamed,
				},
			},
		},
		{
			name: "replace mult with mult, backwards",
			n:    10,
			want: "> 1 2 3 9 9 9 5 6 6 6 7 8 8 10 10 10 10 10",
			actions: []action{
				{
					at:     9,
					target: 4,
					op:     nject.ReplaceNamed,
				},
			},
		},
		{
			name: "replace mult with mult, forwards, gapped",
			n:    10,
			want: "> 1 2 3 4 4 5 7 8 8 6 6 6 10 10 10 10 10",
			actions: []action{
				{
					at:     6,
					target: 9,
					op:     nject.ReplaceNamed,
				},
			},
		},
		{
			name: "replace mult with mult, forwards, adjacent",
			n:    10,
			want: "> 1 2 3 4 4 5 6 6 6 7 8 8 10 10 10 10 10",
			actions: []action{
				{
					at:     8,
					target: 9,
					op:     nject.ReplaceNamed,
				},
			},
		},
		{
			name: "replace mult with mult, backwards, adjacent",
			n:    10,
			want: "> 1 2 3 4 4 5 6 6 6 7 9 9 9 10 10 10 10 10",
			actions: []action{
				{
					at:     9,
					target: 8,
					op:     nject.ReplaceNamed,
				},
			},
		},
		{
			name: "replace mult with mult, backwards, gapped",
			n:    10,
			want: "> 1 2 3 9 9 9 5 6 6 6 7 8 8 10 10 10 10 10",
			actions: []action{
				{
					at:     9,
					target: 4,
					op:     nject.ReplaceNamed,
				},
			},
		},
		{
			name: "multiple moves, before, gapped",
			n:    10,
			want: "> 3 1 6 6 6 2 4 4 5 7 8 8 9 9 9 10 10 10 10 10",
			actions: []action{
				{at: 3, target: 1, op: nject.InsertBeforeNamed},
				{at: 6, target: 2, op: nject.InsertBeforeNamed},
			},
		},
		{
			name: "multiple moves, after, gapped A1",
			n:    10,
			want: "> 2 3 1 4 4 5 6 6 6 7 8 8 9 9 9 10 10 10 10 10",
			actions: []action{
				{at: 1, target: 3, op: nject.InsertAfterNamed},
			},
		},
		{
			name: "multiple moves, after, gapped A2",
			n:    10,
			want: "> 2 3 1 5 6 6 6 7 4 4 8 8 9 9 9 10 10 10 10 10",
			actions: []action{
				{at: 1, target: 3, op: nject.InsertAfterNamed},
				{at: 4, target: 7, op: nject.InsertAfterNamed},
			},
		},
		{
			name: "multiple moves, after, gapped A3",
			n:    10,
			want: "> 2 3 1 6 6 6 7 4 4 8 8 9 9 9 5 10 10 10 10 10",
			actions: []action{
				{at: 1, target: 3, op: nject.InsertAfterNamed},
				{at: 4, target: 7, op: nject.InsertAfterNamed},
				{at: 5, target: 9, op: nject.InsertAfterNamed},
			},
		},
		{
			name: "mixed up moves",
			n:    12,
			want: "> 3 1 4 4 5 6 6 6 9 9 9 8 8 2 7 10 10 10 10 10 11 12 12 12",
			actions: []action{
				{at: 2, target: 9, op: nject.InsertAfterNamed},
				{at: 3, target: 1, op: nject.InsertBeforeNamed},
				{at: 6, target: 7, op: nject.InsertAfterNamed},
				{at: 7, target: 2, op: nject.InsertAfterNamed},
				{at: 9, target: 8, op: nject.InsertBeforeNamed},
				{at: 10, target: 7, op: nject.InsertAfterNamed},   // no-op
				{at: 11, target: 12, op: nject.InsertBeforeNamed}, // no-op
			},
		},
		{
			name: "moved and replaced together",
			n:    10,
			want: "> 1 2 3 8 8 5 9 9 9 10 10 10 10 10 4 4",
			actions: []action{
				{at: 4, target: 7, op: nject.ReplaceNamed},
				{at: 8, target: 3, op: nject.InsertAfterNamed},
				{at: 9, target: 6, op: nject.ReplaceNamed},
				{at: 10, target: 4, op: nject.InsertBeforeNamed},
			},
		},
	}
	// want: "> 1 2 3 4 4 5 6 6 6 7 8 8 9 9 9 10 10 10 10 10",
	mkInjector := func(prefix string, i int) nject.Provider {
		return nject.Provide(prefix+strconv.Itoa(i), func(s string) string {
			return s + " " + strconv.Itoa(i)
		})
	}
	mkInjectors := func(count int, i int) *nject.Collection {
		injectors := make([]interface{}, 0, count)
		n := strconv.Itoa(count) + "@" + strconv.Itoa(i)
		for j := 1; j <= count; j++ {
			injectors = append(injectors, mkInjector(n, i))
		}
		return nject.Sequence(n, injectors...)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			injectors := []interface{}{
				nject.Provide("X0", func() string { return ">" }),
			}
			for i := 1; i <= tc.n; i++ {
				var injector interface{}
				for _, m := range []int{7, 5, 3, 2} {
					if i > m && i%m == 0 {
						injector = mkInjectors(m, i)
						break
					}
				}
				if injector == nil {
					injector = mkInjector(strconv.Itoa(i), i)
				}
				injector = nject.Provide("X"+strconv.Itoa(i), injector)
				for _, action := range tc.actions {
					if action.at == i {
						injector = action.op("X"+strconv.Itoa(action.target), injector)
					}
				}
				injectors = append(injectors, injector)
			}
			var ran bool
			err := nject.Run(t.Name(),
				nject.Sequence("test", injectors...),
				func(s string) {
					ran = true
					assert.Equal(t, tc.want, s)
				},
			)
			require.NoError(t, err, "run")
			assert.True(t, ran)
		})
	}
}
