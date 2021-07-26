package nject

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

type FillStruct struct {
	x   int
	s0  s0            `t1:"-"`
	s1  s1            `t1:"ignore"`
	S2  s2            `t1:"fill"`
	Sub FillSubStruct `t1:"fields" t2:"whole" t3:"ignore"`
}

type FillSubStruct struct {
	y  int
	S3 s3 `t1:"fill"`
}

type FillStruct2 struct{ FillStruct }
type FillStruct3 struct{ FillStruct }
type FillStruct4 struct{ FillStruct }
type FillStruct5 struct{ FillStruct }

func TestFiller(t *testing.T) {
	t.Parallel()
	var called bool
	err := Run("TestFiller",
		s0("s0"),
		s1("s1"),
		s2("s2"),
		s3("s3"),
		func() *FillStruct3 {
			return &FillStruct3{FillStruct{s0: "i", x: 3}}
		},
		func() FillSubStruct {
			return FillSubStruct{y: 8, S3: "j"}
		},
		MustMakeStructBuilder(FillStruct{}, WithTag("")),
		MustMakeStructBuilder(&FillStruct2{}, WithTag("t1")),
		MustMakeStructFiller(&FillStruct3{}, WithTag("")),
		MustMakeStructBuilder(&FillStruct4{}, WithTag("t2")),
		MustMakeStructBuilder(&FillStruct5{}, WithTag("t3")),
		func(f1 FillStruct, f2 *FillStruct2, f3 *FillStruct3, f4 *FillStruct4, f5 *FillStruct5) {
			called = true
			assert.Equal(t, s0(""), f1.s0, "f1.s0 not filled")
			assert.Equal(t, s1(""), f1.s1, "f1.s1 not filled")
			assert.Equal(t, s2("s2"), f1.S2, "f1.s2 filled")
			assert.Equal(t, s3("s3"), f1.Sub.S3, "f1.s3 filled")

			assert.Equal(t, s0(""), f2.s0, "f2.s0 not filled")
			assert.Equal(t, s1(""), f2.s1, "f2.s1 not filled")
			assert.Equal(t, s2("s2"), f2.S2, "f2.s2 filled")
			assert.Equal(t, s3("s3"), f2.Sub.S3, "f2.s3 filled")

			assert.Equal(t, 3, f3.x, "f3.x not filled")
			assert.Equal(t, s0("i"), f3.s0, "f3.s0 not filled")
			assert.Equal(t, s1(""), f3.s1, "f3.s1 not filled")
			assert.Equal(t, s2("s2"), f3.S2, "f3.s2 filled")
			assert.Equal(t, s3("s3"), f3.Sub.S3, "f3.s3 filled")

			assert.Equal(t, 3, f3.x, "f3.x not filled")
			assert.Equal(t, s0("i"), f3.s0, "f3.s0 not filled")
			assert.Equal(t, s1(""), f3.s1, "f3.s1 not filled")
			assert.Equal(t, s2("s2"), f3.S2, "f3.s2 filled")

			assert.Equal(t, s3("j"), f4.Sub.S3, "f4.s3 from sub")
			assert.Equal(t, s3(""), f5.Sub.S3, "f5.s3 not filled")
		},
	)
	assert.NoError(t, err)
	assert.True(t, called)
}

type funcWrapper struct {
	v reflect.Value
}

func (f funcWrapper) NumIn() int                              { return f.v.Type().NumIn() }
func (f funcWrapper) In(i int) reflect.Type                   { return f.v.Type().In(i) }
func (f funcWrapper) NumOut() int                             { return f.v.Type().NumOut() }
func (f funcWrapper) Out(i int) reflect.Type                  { return f.v.Type().Out(i) }
func (f funcWrapper) Call(in []reflect.Value) []reflect.Value { return f.v.Call(in) }

func TestReflective(t *testing.T) {
	t.Parallel()
	var called bool
	var fCalled bool
	f := func(inner func(), _ s1) {
		fCalled = true
		assert.False(t, called, "before inner")
		inner()
		assert.True(t, called, "after inner")
	}
	w := funcWrapper{v: reflect.ValueOf(f)}
	_ = Run("TestReflective",
		s1("s1"),
		w,
		func() {
			called = true
		},
	)
	assert.True(t, fCalled, "f called")
}
