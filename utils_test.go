package nject

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSaveTo(t *testing.T) {
	f := func() {}
	cases := []struct {
		want  string
		thing interface{}
	}{
		{
			want: "not a valid pointer",
		},
		{
			want:  "is nil",
			thing: (*string)(nil),
		},
		{
			want:  "is not a pointer",
			thing: 7,
		},
		{
			want:  "may not be a pointer to a function",
			thing: &f,
		},
	}
	for _, tc := range cases {
		t.Log(tc.want)
		_, err := SaveTo(tc.thing)
		if assert.Error(t, err, tc.want) {
			assert.Contains(t, err.Error(), tc.want)
		}
	}
}
