package nject

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStripUnused(t *testing.T) {
	cases := []struct {
		name  string
		input []reflect.Type
		want  []reflect.Type
	}{
		{
			name:  "empty",
			input: []reflect.Type{},
			want:  []reflect.Type{},
		},
		{
			name:  "one junk",
			input: []reflect.Type{unusedType},
			want:  []reflect.Type{},
		},
		{
			name:  "two junk",
			input: []reflect.Type{unusedType, unusedType},
			want:  []reflect.Type{},
		},
		{
			name:  "start unused",
			input: []reflect.Type{unusedType, errorType},
			want:  []reflect.Type{errorType},
		},
		{
			name:  "end unused",
			input: []reflect.Type{errorType, unusedType},
			want:  []reflect.Type{errorType},
		},
		{
			name:  "mid unused",
			input: []reflect.Type{errorType, unusedType, terminalErrorType},
			want:  []reflect.Type{errorType, terminalErrorType},
		},
		{
			name:  "alt unused",
			input: []reflect.Type{errorType, unusedType, terminalErrorType, unusedType},
			want:  []reflect.Type{errorType, terminalErrorType},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := stripUnused(tc.input)
			assert.Equal(t, tc.want, got, "types")
			gotCodes := stripUnusedCodes(toTypeCodes(tc.input))
			assert.Equal(t, toTypeCodes(tc.want), gotCodes, "typeCodes")
		})
	}
}

func TestUnused(t *testing.T) {
	var called bool
	var callCount int

	var invoke1 func() Unused
	var invoke2 func()
	var init2 func() Unused

	cases := []struct {
		name          string
		initF         any
		doInit        func()
		invokeF       any
		doInvoke      func()
		chain         []any
		invalid       bool
		callsExpected int
	}{
		{
			name: "add unused injector",
			chain: []any{
				17,
				func(_ int, _ Unused) {
					called = true
				},
			},
		},
		{
			name: "add unused responder",
			chain: []any{
				Required(func(inner func() Unused) {
					_ = inner()
					called = true
				}),
				func() {},
			},
		},
		{
			name:    "invoke wants unused",
			invokeF: &invoke1,
			doInvoke: func() {
				_ = invoke1()
			},
			chain: []any{
				func() { called = true },
			},
		},
		{
			name:    "init wants unused",
			invokeF: &invoke2,
			doInvoke: func() {
				invoke1()
			},
			initF: &init2,
			doInit: func() {
				_ = init2()
			},
			chain: []any{
				func() { called = true },
			},
		},
		{
			name: "must consume Unused",
			chain: []any{
				MustConsume(Required(func() Unused { called = true; return Unused{} })),
				func() {},
			},
		},
		{
			name: "must consume not used",
			chain: []any{
				MustConsume(Required(func() int { called = true; return 7 })),
				func() {},
			},
			invalid: true,
		},
		{
			name: "must consume provided down, wanted up",
			chain: []any{
				MustConsume(Required(func() Unused { callCount++; return Unused{} })),
				Required(func(inner func() Unused) {
					_ = inner()
					callCount++
				}),
				func() { callCount++ },
			},
			callsExpected: 3,
		},
		{
			name: "returning nothing",
			chain: []any{
				func() { called = true },
				func() {},
			},
		},
		{
			name: "returning just Unused counts like returning nothing",
			chain: []any{
				func() Unused {
					called = true
					return Unused{}
				},
				func() {},
			},
		},
		{
			name: "provided by init, returned by invoke",
			chain: []any{
				func() { called = true },
			},
			initF: &init2,
			doInit: func() {
				_ = init2()
			},
			invokeF: &invoke1,
			doInvoke: func() {
				_ = invoke1()
			},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			called = false
			callCount = 0
			var err error
			if tc.initF == nil && tc.invokeF == nil {
				err = Run(tc.name, tc.chain...)
			} else {
				c := Sequence(tc.name, tc.chain...)
				err = c.Bind(tc.invokeF, tc.initF)
				if err == nil && !tc.invalid {
					if tc.doInit != nil {
						tc.doInit()
					}
					tc.doInvoke()
				}
			}
			if err != nil {
				t.Logf("error is %+v", err)
			}
			if tc.invalid {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tc.callsExpected != 0 {
					assert.Equal(t, tc.callsExpected, callCount, "calls")
				} else {
					assert.True(t, called)
				}
			}
		})
	}
}
