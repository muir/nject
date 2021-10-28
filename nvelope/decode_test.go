package nvelope_test

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/muir/nject/nvelope"
	"github.com/stretchr/testify/assert"
)

type Complex128 complex128

func (c Complex128) MarshalText() ([]byte, error) {
	return []byte(fmt.Sprint(c)), nil
}

type Complex64 complex64

func (c Complex64) MarshalText() ([]byte, error) {
	return []byte(fmt.Sprint(c)), nil
}

func TestDecodeQuerySimpleParameters(t *testing.T) {
	do := captureOutput("/x", func(s struct {
		Int        int         `json:",omitempty" nvelope:"query,name=int"`
		Int8       int8        `json:",omitempty" nvelope:"query,name=int8"`
		Int16      int16       `json:",omitempty" nvelope:"query,name=int16"`
		Int32      int32       `json:",omitempty" nvelope:"query,name=int32"`
		Int64      int64       `json:",omitempty" nvelope:"query,name=int64"`
		Uint       uint        `json:",omitempty" nvelope:"query,name=uint"`
		Uint8      uint8       `json:",omitempty" nvelope:"query,name=uint8"`
		Uint16     uint16      `json:",omitempty" nvelope:"query,name=uint16"`
		Uint32     uint32      `json:",omitempty" nvelope:"query,name=uint32"`
		Uint64     uint64      `json:",omitempty" nvelope:"query,name=uint64"`
		Float32    float32     `json:",omitempty" nvelope:"query,name=float32"`
		Float64    float64     `json:",omitempty" nvelope:"query,name=float64"`
		String     string      `json:",omitempty" nvelope:"query,name=string"`
		IntP       *int        `json:",omitempty" nvelope:"query,name=intp"`
		Int8P      *int8       `json:",omitempty" nvelope:"query,name=int8p"`
		Int16P     *int16      `json:",omitempty" nvelope:"query,name=int16p"`
		Int32P     *int32      `json:",omitempty" nvelope:"query,name=int32p"`
		Int64P     *int64      `json:",omitempty" nvelope:"query,name=int64p"`
		UintP      *uint       `json:",omitempty" nvelope:"query,name=uintp"`
		Uint8P     *uint8      `json:",omitempty" nvelope:"query,name=uint8p"`
		Uint16P    *uint16     `json:",omitempty" nvelope:"query,name=uint16p"`
		Uint32P    *uint32     `json:",omitempty" nvelope:"query,name=uint32p"`
		Uint64P    *uint64     `json:",omitempty" nvelope:"query,name=uint64p"`
		Float32P   *float32    `json:",omitempty" nvelope:"query,name=float32p"`
		Float64P   *float64    `json:",omitempty" nvelope:"query,name=float64p"`
		StringP    *string     `json:",omitempty" nvelope:"query,name=stringp"`
		Complex64  *Complex64  `json:",omitempty" nvelope:"query,name=complex64"`
		Complex128 *Complex128 `json:",omitempty" nvelope:"query,name=complex128"`
		BoolP      *bool       `json:",omitempty" nvelope:"query,name=boolp"`
	}) (nvelope.Response, error) {
		return s, nil
	})
	assert.Equal(t, `200->{"Int":135}`, do("/x?int=135", ""))
	assert.Equal(t, `200->{"Int8":-5}`, do("/x?int8=-5", ""))
	assert.Equal(t, `200->{"Int16":127}`, do("/x?int16=127", ""))
	assert.Equal(t, `200->{"Int32":11}`, do("/x?int32=11", ""))
	assert.Equal(t, `200->{"Int64":-38}`, do("/x?int64=-38", ""))
	assert.Equal(t, `200->{"Uint":135}`, do("/x?uint=135", ""))
	assert.Equal(t, `200->{"Uint8":5}`, do("/x?uint8=5", ""))
	assert.Equal(t, `200->{"Uint16":127}`, do("/x?uint16=127", ""))
	assert.Equal(t, `200->{"Uint32":11}`, do("/x?uint32=11", ""))
	assert.Equal(t, `200->{"Uint64":38}`, do("/x?uint64=38", ""))
	assert.Equal(t, `200->{"Float64":38.7}`, do("/x?float64=38.7", ""))
	assert.Equal(t, `200->{"Float32":11.1}`, do("/x?float32=11.1", ""))
	assert.Equal(t, `200->{"String":"fred"}`, do("/x?string=fred", ""))
	assert.Equal(t, `200->{"IntP":135}`, do("/x?intp=135", ""))
	assert.Equal(t, `200->{"Int8P":-5}`, do("/x?int8p=-5", ""))
	assert.Equal(t, `200->{"Int16P":127}`, do("/x?int16p=127", ""))
	assert.Equal(t, `200->{"Int32P":11}`, do("/x?int32p=11", ""))
	assert.Equal(t, `200->{"Int64P":-38}`, do("/x?int64p=-38", ""))
	assert.Equal(t, `200->{"UintP":135}`, do("/x?uintp=135", ""))
	assert.Equal(t, `200->{"Uint8P":5}`, do("/x?uint8p=5", ""))
	assert.Equal(t, `200->{"Uint16P":127}`, do("/x?uint16p=127", ""))
	assert.Equal(t, `200->{"Uint32P":11}`, do("/x?uint32p=11", ""))
	assert.Equal(t, `200->{"Uint64P":38}`, do("/x?uint64p=38", ""))
	assert.Equal(t, `200->{"Float64P":38.7}`, do("/x?float64p=38.7", ""))
	assert.Equal(t, `200->{"Float32P":11.1}`, do("/x?float32p=11.1", ""))
	assert.Equal(t, `200->{"StringP":"fred"}`, do("/x?stringp=fred", ""))
	assert.Equal(t, `200->{"Complex64":"(38.7-9.3i)"}`, do("/x?complex64="+url.QueryEscape("38.7-9.3i"), ""))
	assert.Equal(t, `200->{"Complex128":"(11.1+22.1i)"}`, do("/x?complex128="+url.QueryEscape("11.1+22.1i"), ""))
	assert.Equal(t, `200->{"BoolP":false}`, do("/x?boolp=false", ""))
}

func TestDecodeQueryComplexParameters(t *testing.T) {
	do := captureOutput("/x", func(s struct {
		IntSlice     []int          `json:",omitempty" nvelope:"query,name=intslice,explode=false"`
		Int8Slice    []*int8        `json:",omitempty" nvelope:"query,name=int8slice,explode=true"`
		Int16Slice   []*int8        `json:",omitempty" nvelope:"query,name=int16slice,explode=false,delimiter=space"`
		Int32Slice   *[]*int8       `json:",omitempty" nvelope:"query,name=int32slice,explode=false,delimiter=pipe"`
		MapIntBool   map[int]bool   `json:",omitempty" nvelope:"query,name=mapintbool,explode=false"`
		MapIntString map[int]string `json:",omitempty" nvelope:"query,name=mapintstring,deepObject=true"`
		Emb1         *struct {
			Int    int    `json:",omitempty" nvelope:"eint"`
			Int8   int8   `json:",omitempty" nvelope:"eint8"`
			Int16  int16  `json:",omitempty" nvelope:"eint16"`
			String string `json:",omitempty"`
		} `json:",omitempty" nvelope:"query,name=emb1,explode=false"`
		Emb2 *struct {
			Int    int    `json:",omitempty" nvelope:"eint"`
			Int8   int8   `json:",omitempty" nvelope:"eint8"`
			Int16  int16  `json:",omitempty" nvelope:"eint16"`
			String string `json:",omitempty"`
		} `json:",omitempty" nvelope:"query,name=emb2,deepObject=true"`
	}) (nvelope.Response, error) {
		return s, nil
	})
	assert.Equal(t, `200->{"IntSlice":[1,7]}`, do("/x?intslice=1,7", ""))
	assert.Equal(t, `200->{"Int8Slice":[10,11,12]}`, do("/x?int8slice=10&int8slice=11&int8slice=12", ""))
	assert.Equal(t, `200->{"Int16Slice":[8,22,-3]}`, do("/x?int16slice=8%2022%20-3", ""))
	assert.Equal(t, `200->{"Int32Slice":[7,11,13]}`, do("/x?int32slice=7|11|13", ""))
	assert.Equal(t, `200->{"MapIntBool":{"-9":false,"7":true}}`, do("/x?mapintbool=7,true,-9,false", ""))
	assert.Equal(t, `200->{"MapIntString":{"-9":"hi","7":"bye"}}`, do("/x?mapintstring[7]=bye&mapintstring[-9]=hi", ""))
	assert.Equal(t, `200->{"Emb1":{"Int":192,"Int8":-3,"String":"foo"}}`, do("/x?emb1=eint,192,eint8,-3,String,foo", ""))
	assert.Equal(t, `200->{"Emb2":{"Int":193,"Int8":-4,"String":"bar"}}`, do("/x?emb2[eint]=193&emb2[eint8]=-4&emb2[String]=bar", ""))
}

type Foo string

func (fp *Foo) UnmarshalText(b []byte) error {
	*fp = Foo("~" + string(b) + "~")
	return nil
}

func TestDecodeQueryJSONParameters(t *testing.T) {
	do := captureOutput("/x", func(s struct {
		Foo  Foo      `json:",omitempty" nvelope:"query,name=foo,explode=false"`
		FooP *Foo     `json:",omitempty" nvelope:"query,name=foop,explode=false"`
		FooA []Foo    `json:",omitempty" nvelope:"query,name=fooa,explode=true"`
		FooB *[]*Foo  `json:",omitempty" nvelope:"query,name=foob,explode=false"`
		S1   string   `json:",omitempty" nvelope:"query,name=s1,content=application/json"`
		S2   *string  `json:",omitempty" nvelope:"query,name=s2,content=application/json"`
		S3   **string `json:",omitempty" nvelope:"query,name=s3,content=application/json"`
	}) (nvelope.Response, error) {
		return s, nil
	})
	assert.Equal(t, `200->{"Foo":"~bar~"}`, do("/x?foo=bar", ""))
	assert.Equal(t, `200->{"FooP":"~baz~"}`, do("/x?foop=baz", ""))
	assert.Equal(t, `200->{"FooA":["~bar~","~baz~"]}`, do("/x?fooa=bar&fooa=baz", ""))
	assert.Equal(t, `200->{"FooB":["~bing~","~baz~"]}`, do("/x?foob=bing,baz", ""))
	assert.Equal(t, `200->{"S1":"doof"}`, do(`/x?s1="doof"`, ""))
	assert.Equal(t, `200->{"S2":"boor"}`, do(`/x?s2="boor"`, ""))
	assert.Equal(t, `200->{"S3":"ppp"}`, do(`/x?s3="ppp"`, ""))
}
