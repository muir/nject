package nject

import (
	"reflect"
	"testing"

	v1 "github.com/muir/nject/internal/foo"
	v2 "github.com/muir/nject/internal/foo/v2"

	"github.com/stretchr/testify/assert"
)

func TestVersionedNames(t *testing.T) {
	x1 := getTypeCode(v1.Bar{})
	assert.Equal(t, "foo.Bar", x1.String())
	t.Log("base type", reflect.TypeOf(v2.Bar{}).PkgPath())
	assert.Equal(t, "foo/v2.Bar", getTypeCode(v2.Bar{}).String())
	t.Log("pointer", reflect.TypeOf(&v2.Bar{}).PkgPath())
	assert.Equal(t, "*foo.Bar", getTypeCode(&v2.Bar{}).String()) // :(
	assert.Equal(t, "*foo.Bar", getTypeCode(&v1.Bar{}).String()) // :(
	t.Log("slice", reflect.TypeOf([]v2.Bar{}).PkgPath())
	assert.Equal(t, "[]foo.Bar", getTypeCode([]v2.Bar{}).String()) // :(
	assert.Equal(t, "[]foo.Bar", getTypeCode([]v1.Bar{}).String()) // :(
	dups := duplicateTypes()
	assert.Contains(t, dups, "[]foo.Bar")
	assert.Contains(t, dups, "*foo.Bar")
	t.Log("duplicates", dups)
}
