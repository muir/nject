package nject

import (
	"reflect"
	"strconv"
	"strings"
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
			name: "multiples-yes",
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
			name: "multiples-no",
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

func TestCacheSizes(t *testing.T) {
	type CS00 string
	type CS01 string
	type CS02 string
	type CS03 string
	type CS04 string
	type CS05 string
	type CS06 string
	type CS07 string
	type CS08 string
	type CS09 string

	type CS10 string
	type CS11 string
	type CS12 string
	type CS13 string
	type CS14 string
	type CS15 string
	type CS16 string
	type CS17 string
	type CS18 string
	type CS19 string

	type CS20 string
	type CS21 string
	type CS22 string
	type CS23 string
	type CS24 string
	type CS25 string
	type CS26 string
	type CS27 string
	type CS28 string
	type CS29 string

	type CS30 string
	type CS31 string
	type CS32 string
	type CS33 string
	type CS34 string
	type CS35 string
	type CS36 string
	type CS37 string
	type CS38 string
	type CS39 string

	all := Cacheable(Sequence("all",
		func() CS00 { return "01" },
		func() CS01 { return "02" },
		func() CS02 { return "03" },
		func() CS03 { return "04" },
		func() CS04 { return "05" },
		func() CS05 { return "06" },
		func() CS06 { return "07" },
		func() CS07 { return "08" },
		func() CS08 { return "09" },
		func() CS09 { return "10" },

		func() CS10 { return "11" },
		func() CS11 { return "12" },
		func() CS12 { return "13" },
		func() CS13 { return "14" },
		func() CS14 { return "15" },
		func() CS15 { return "16" },
		func() CS16 { return "17" },
		func() CS17 { return "18" },
		func() CS18 { return "19" },
		func() CS19 { return "10" },

		func() CS20 { return "21" },
		func() CS21 { return "22" },
		func() CS22 { return "23" },
		func() CS23 { return "24" },
		func() CS24 { return "25" },
		func() CS25 { return "26" },
		func() CS26 { return "27" },
		func() CS27 { return "28" },
		func() CS28 { return "29" },
		func() CS29 { return "20" },

		func() CS30 { return "31" },
		func() CS31 { return "32" },
		func() CS32 { return "33" },
		func() CS33 { return "34" },
		func() CS34 { return "35" },
		func() CS35 { return "36" },
		func() CS36 { return "37" },
		func() CS37 { return "38" },
		func() CS38 { return "39" },
		func() CS39 { return "30" },
	))

	var rc1 int
	var rc2 int
	var rc3 int
	var rc4 int

	rw1 := NotCacheable(Memoize(func(s00 CS00) string {
		rc1++
		return strings.Join([]string{string(s00), strconv.Itoa(rc1)}, "-")
	}))
	rw2 := NotCacheable(Memoize(func(s string, s00 CS00, s01 CS01, s02 CS02) string {
		rc2++
		return strings.Join([]string{s, string(s00), string(s01), string(s02), strconv.Itoa(rc2)}, "-")
	}))
	rw3 := NotCacheable(Memoize(func(s string,
		s00 CS00, s01 CS01, s02 CS02,
		s03 CS03, s04 CS04, s05 CS05,
		s06 CS06, s07 CS07, s08 CS08,
		s09 CS09, s17 CS17, s18 CS18,
	) string {
		rc3++
		return strings.Join([]string{s,
			string(s00), string(s01), string(s02),
			string(s03), string(s04), string(s05),
			string(s06), string(s07), string(s08),
			string(s09), string(s17), string(s18),
			strconv.Itoa(rc3)}, "+")
	}))
	rw4 := NotCacheable(Memoize(func(s string,
		s00 CS00, s01 CS01, s02 CS02, s03 CS03, s04 CS04, s05 CS05, s06 CS06, s07 CS07, s08 CS08, s09 CS09,
		s10 CS10, s11 CS11, s12 CS12, s13 CS13, s14 CS14, s15 CS15, s16 CS16, s17 CS17, s18 CS18, s19 CS19,
		s20 CS20, s21 CS21, s22 CS22, s23 CS23, s24 CS24, s25 CS25, s26 CS26, s27 CS27, s28 CS28, s29 CS29,
		s30 CS30, s31 CS31, s32 CS32, s33 CS33, s34 CS34, s35 CS35, s36 CS36, s37 CS37, s38 CS38, s39 CS39,
	) string {
		rc4++
		return strings.Join([]string{s,
			string(s00), string(s01), string(s02), string(s03), string(s04), string(s05), string(s06), string(s07), string(s08), string(s09),
			string(s10), string(s11), string(s12), string(s13), string(s14), string(s15), string(s16), string(s17), string(s18), string(s19),
			string(s20), string(s21), string(s22), string(s23), string(s24), string(s25), string(s26), string(s27), string(s28), string(s29),
			string(s30), string(s31), string(s32), string(s33), string(s34), string(s35), string(s36), string(s37), string(s38), string(s39),
			strconv.Itoa(rc4)}, "/")
	}))

	var prior string
	var count int
	var didCompare bool
	var dbg *Debugging

	tseq := Sequence("test",
		all,
		rw1, rw2, rw3, rw4,

		func(s string, d *Debugging) {
			assert.Greater(t, len(s), 50, "length")
			if count == 0 {
				prior = s
			} else {
				assert.Equal(t, prior, s, "repeats")
				didCompare = true
			}
			dbg = d
			count++
		},
	)

	var f func()
	tseq.MustBind(&f, nil)
	f()
	assert.False(t, didCompare, "no compare yet")
	f()
	assert.True(t, didCompare, "now compared")
	f()
	f()
	t.Log(strings.Join(dbg.Included, "\n"))
}
