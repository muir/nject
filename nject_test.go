package nject

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAnnotateCacheableProvider(t *testing.T) {
	wrapTest(t, func(t *testing.T) {
		var counter int
		p := Cacheable(Provide("foo", func() { counter++ }))
		require.IsType(t, &provider{}, p)
		require.True(t, p.(*provider).cacheable)
		require.True(t, p.(*provider).copy().cacheable)
	})
}

func TestAnnotateCacheableSequence(t *testing.T) {
	wrapTest(t, func(t *testing.T) {
		var counter int
		c := Cacheable(Sequence("foo", func() { counter++ }))
		require.IsType(t, &Collection{}, c)
		require.True(t, c.(*Collection).contents[0].cacheable)
	})
}

func TestAnnotateCacheableBare(t *testing.T) {
	wrapTest(t, func(t *testing.T) {
		var counter int
		p := Cacheable(func() { counter++ })
		require.IsType(t, &provider{}, p)
		require.True(t, p.(*provider).cacheable)
		require.True(t, p.(*provider).copy().cacheable)
	})
}

func TestAnnotateNotCacheable(t *testing.T) {
	wrapTest(t, func(t *testing.T) {
		var counter int
		p := NotCacheable(func() { counter++ })
		require.IsType(t, &provider{}, p)
		require.True(t, p.(*provider).notCacheable)
		require.True(t, p.(*provider).copy().notCacheable)
	})
}

func TestAnnotateMustConsume(t *testing.T) {
	stc := getTypeCode("foo")
	wrapTest(t, func(t *testing.T) {
		p := MustConsume[string](func() {})
		require.IsType(t, &provider{}, p)
		require.NotNil(t, p.(*provider).mustConsume)
		require.Contains(t, p.(*provider).mustConsume, stc)
		require.NotNil(t, p.(*provider).copy().mustConsume)
		require.Contains(t, p.(*provider).copy().mustConsume, stc)
	})
}

func TestAnnotateConsumptionOptional(t *testing.T) {
	stc := getTypeCode("foo")
	wrapTest(t, func(t *testing.T) {
		p := ConsumptionOptional[string](func() {})
		require.IsType(t, &provider{}, p)
		require.NotNil(t, p.(*provider).consumptionOptional)
		require.Contains(t, p.(*provider).consumptionOptional, stc)
		require.NotNil(t, p.(*provider).copy().consumptionOptional)
		require.Contains(t, p.(*provider).copy().consumptionOptional, stc)
	})
}

func TestAnnotateLoose(t *testing.T) {
	stc := getTypeCode("foo")
	wrapTest(t, func(t *testing.T) {
		var counter int
		p := Loose[string](func() { counter++ })
		require.IsType(t, &provider{}, p)
		require.NotNil(t, p.(*provider).loose)
		require.NotNil(t, p.(*provider).copy().loose)
		require.Contains(t, p.(*provider).loose, stc)
		require.Contains(t, p.(*provider).copy().loose, stc)
	})
}

func TestAnnotateShadowing(t *testing.T) {
	stc := getTypeCode("foo")
	wrapTest(t, func(t *testing.T) {
		var counter int
		p := AllowReturnShadowing[string](func() { counter++ })
		require.IsType(t, &provider{}, p)
		require.NotNil(t, p.(*provider).shadowingAllowed)
		require.Contains(t, p.(*provider).shadowingAllowed, stc)
		require.NotNil(t, p.(*provider).copy().shadowingAllowed)
		require.Contains(t, p.(*provider).copy().shadowingAllowed, stc)
	})
}

func TestAnnotateDesired(t *testing.T) {
	wrapTest(t, func(t *testing.T) {
		var counter int
		p := Desired(Provide("foo", func() { counter++ }))
		require.IsType(t, &provider{}, p)
		require.True(t, p.(*provider).desired)
		require.True(t, p.(*provider).copy().desired)
	})
}

func TestAnnotateRequired(t *testing.T) {
	wrapTest(t, func(t *testing.T) {
		var counter int
		p := Required(Provide("foo", func() { counter++ }))
		require.IsType(t, &provider{}, p)
		require.True(t, p.(*provider).required)
		require.True(t, p.(*provider).copy().required)
	})
}

func TestAnnotateMustCache(t *testing.T) {
	wrapTest(t, func(t *testing.T) {
		var counter int
		p := MustCache(Provide("foo", func() { counter++ }))
		require.IsType(t, &provider{}, p)
		require.True(t, p.(*provider).mustCache)
		require.True(t, p.(*provider).cacheable)
		require.True(t, p.(*provider).copy().mustCache)
	})
}

func TestAnnotateCombo(t *testing.T) {
	wrapTest(t, func(t *testing.T) {
		var counter int
		p := Required(MustCache(Provide("foo", func() { counter++ })))
		require.IsType(t, &provider{}, p)
		require.True(t, p.(*provider).mustCache)
		require.True(t, p.(*provider).cacheable)
		require.True(t, p.(*provider).required)
		require.True(t, p.(*provider).copy().mustCache)
	})
}

func TestAnnotateMemoize(t *testing.T) {
	wrapTest(t, func(t *testing.T) {
		var counter int
		p := Memoize(Provide("foo", func() { counter++ }))
		require.IsType(t, &provider{}, p)
		require.True(t, p.(*provider).memoize)
		require.True(t, p.(*provider).cacheable)
		require.True(t, p.(*provider).copy().memoize)
	})
}

func TestAnnotateCallsinner(t *testing.T) {
	wrapTest(t, func(t *testing.T) {
		var counter int
		p := callsInner(Provide("foo", func() { counter++ }))
		require.IsType(t, &provider{}, p)
		require.True(t, p.(*provider).callsInner)
		require.True(t, p.(*provider).copy().callsInner)
	})
}
