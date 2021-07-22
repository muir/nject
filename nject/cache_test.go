package nject

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

type imt1 interface {
	x() int
}

type Iimt1 int

func (i Iimt1) x() int {
	return int(i)
}

type aimt1 []int

func (i aimt1) x() int {
	return i[0]
}

type smt struct {
	Imt imt1
}

type outer struct {
	IgnoreMe int
	Inner    smt
}

func castToImt1(i interface{}) imt1 {
	return i.(imt1)
}

func TestCanBeMapKey(t *testing.T) {
	var nilimt imt1
	iimt := castToImt1(Iimt1(7))
	aimt := castToImt1(aimt1([]int{10, 11}))
	cases := []struct {
		name         string
		values       []interface{}
		simple       bool // true if function should be nil
		typeOverride []interface{}
		mappable     bool
	}{
		{
			name:     "array",
			values:   []interface{}{[2]int{3, 7}, "foo"},
			simple:   true,
			mappable: true,
		},
		{
			name: "nil imp",
			values: []interface{}{smt{
				Imt: nilimt,
			}},
			simple:   false,
			mappable: true,
		},
		{
			name: "int imp",
			values: []interface{}{smt{
				Imt: iimt,
			}},
			simple:   false,
			mappable: true,
		},
		{
			name: "slice imp",
			values: []interface{}{smt{
				Imt: aimt,
			}},
			simple:   false,
			mappable: false,
		},
		{
			name: "slice imp",
			values: []interface{}{outer{
				Inner: smt{
					Imt: iimt,
				},
			}},
			simple:   false,
			mappable: true,
		},
		{
			name: "mutliples-yes",
			values: []interface{}{
				outer{
					Inner: smt{
						Imt: iimt,
					},
				},
				smt{
					Imt: iimt,
				},
			},
			simple:   false,
			mappable: true,
		},
		{
			name: "mutliples-no",
			values: []interface{}{
				outer{
					Inner: smt{
						Imt: iimt,
					},
				},
				smt{
					Imt: aimt,
				},
			},
			simple:   false,
			mappable: false,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			va := make([]reflect.Value, len(tc.values))
			ta := make([]reflect.Type, len(tc.values))
			for i, x := range tc.values {
				v := reflect.ValueOf(x)
				va[i] = v
				if len(tc.typeOverride) > 0 {
					v = reflect.ValueOf(tc.typeOverride[i])
				}
				ta[i] = v.Type()
			}
			quick, f := canBeMapKey(ta)
			t.Logf("quick:%v hasFunc:%v", quick, f != nil)
			if tc.simple {
				assert.Nil(t, f, "no func expected")
				assert.Equal(t, tc.mappable, quick)
			} else {
				assert.True(t, quick, "quick, when not simple")
				if assert.NotNil(t, f, "func expected") {
					assert.Equal(t, tc.mappable, f(va), "computed mappable")
				}
			}
		})
	}
}
