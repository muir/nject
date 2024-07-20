package nject

import (
	"net/http"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	testI          int
	di             = &doesI{}
	dj             = &doesJ{}
	interfaceIType = reflect.TypeOf((*interfaceI)(nil)).Elem()
	interfaceJType = reflect.TypeOf((*interfaceJ)(nil)).Elem()
	interfaceKType = reflect.TypeOf((*interfaceK)(nil)).Elem()
)

var (
	requestType        = reflect.TypeOf((**http.Request)(nil)).Elem()
	responseWriterType = reflect.TypeOf((*http.ResponseWriter)(nil)).Elem()
)

var provideSet1 = map[reflect.Type]int{
	requestType:           1,
	responseWriterType:    1,
	reflect.TypeOf(testI): 3,
	reflect.TypeOf(testI): 4,
}

var provideSet2 = map[reflect.Type]int{
	interfaceIType:        1,
	interfaceJType:        2,
	reflect.TypeOf(di):    3,
	reflect.TypeOf(dj):    4,
	reflect.TypeOf(testI): 5,
}

var bestMatchTests = []struct {
	Name    string
	MapData map[reflect.Type]int
	Find    reflect.Type
	Want    reflect.Type
}{
	{
		"responseWriter",
		provideSet1,
		responseWriterType,
		responseWriterType,
	},
	{
		"interface K",
		provideSet2,
		interfaceKType,
		reflect.TypeOf(dj),
	},
}

func TestBestMatch(t *testing.T) {
	wrapTest(t, func(t *testing.T) {
		for _, test := range bestMatchTests {
			test := test
			m := make(interfaceMap)
			for typ, layer := range test.MapData {
				t.Logf("%s: #%d get type code for %v", test.Name, layer, typ)
				m.Add(getTypeCode(typ), layer, &provider{loose: true})
			}
			f := func() {
				for tc, d := range m {
					t.Logf("\tm[%s] = %s (%s) %d", tc.Type(), d.name, d.typeCode.Type(), d.layer)
				}
				got, _, err := m.bestMatch(getTypeCode(test.Find), "searching for "+test.Name)
				require.NoError(t, err)
				assert.Equal(t, test.Want.String(), got.Type().String(), test.Name)
			}
			if test.Want == nil {
				require.Panics(t, f, test.Name)
			} else {
				f()
			}
		}
	})
}
