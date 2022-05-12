package nject

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

type flows interface {
	DownFlows() (inputs []reflect.Type, outputs []reflect.Type)
	UpFlows() (consume []reflect.Type, produce []reflect.Type)
}

func toTypes(real ...interface{}) []reflect.Type {
	types := make([]reflect.Type, len(real))
	for i, r := range real {
		types[i] = reflect.TypeOf(r)
	}
	return types
}

func toStrings(types []reflect.Type) []string {
	s := make([]string, len(types))
	for i, t := range types {
		s[i] = t.String()
	}
	return s
}

func TestFlows(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		provider interface{}
		upIn     []interface{}
		upOut    []interface{}
		downIn   []interface{}
		downOut  []interface{}
	}{
		{
			name:     "basic",
			provider: func(int, string) error { return nil },
			upOut:    []interface{}{(error)(nil)},
			downIn:   []interface{}{3, ""},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f, ok := tc.provider.(flows)
			if !ok {
				f = Provide(tc.name, tc.provider)
			}
			wantDownIn, wantDownOut := toTypes(tc.downIn), toTypes(tc.downOut)
			downIn, downOut := f.DownFlows()
			assert.Equal(t, toStrings(wantDownIn), toStrings(downIn), "down in")
			assert.Equal(t, toStrings(wantDownOut), toStrings(downOut), "down out")
			upIn, upOut := f.UpFlows()
			wantUpIn, wantUpOut := toTypes(tc.upIn), toTypes(tc.upOut)
			assert.Equal(t, toStrings(wantUpIn), toStrings(upIn), "up in")
			assert.Equal(t, toStrings(wantUpOut), toStrings(upOut), "up out")
		})
	}
}
