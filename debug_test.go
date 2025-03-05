package nject

import (
	"fmt"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func debugOn(t *testing.T) {
	debugOutputMu.Lock()
	debuglnHook = func(stuff ...any) {
		t.Log(stuff...)
	}
	debugfHook = func(format string, stuff ...any) {
		t.Logf(format+"\n", stuff...)
	}
	debugOutputMu.Unlock()
	atomic.StoreUint32(&debug, 1)
}

func debugOff() {
	debugOutputMu.Lock()
	debuglnHook = nil
	debugfHook = nil
	debugOutputMu.Unlock()
	atomic.StoreUint32(&debug, 0)
}

func wrapTest(t *testing.T, inner func(*testing.T)) {
	namedWrapTest(t, "", inner)
}

func namedWrapTest(t *testing.T, name string, inner func(*testing.T)) {
	if !t.Run("1st attempt"+name, func(t *testing.T) { inner(t) }) {
		t.Run("2nd attempt"+name, func(t *testing.T) {
			debugOn(t)
			defer debugOff()
			inner(t)
		})
	}
}

func TestDetailedError(t *testing.T) {
	t.Parallel()

	type MyType1 struct {
		Int int
	}
	type MyType2 []MyType1
	type MyType3 *MyType1
	type MyType4 interface {
		String() string
	}
	type MyType5 interface {
		unimplementable()
	}

	err := Run("expected-to-fail",
		Desired(func() MyType1 { return MyType1{} }),
		Shun(func(m MyType1) MyType3 { return &m }),
		Required(func(m MyType3) MyType2 { return []MyType1{*m} }),
		Cacheable(func() int { return 4 }),
		MustCache(func() string { return "foo1" }),
		Cluster("c1",
			Singleton(func(i int) int64 { return int64(i) }),
			Loose(func(m MyType4) string { return m.String() }),
		),
		Cluster("c2",
			Reorder(time.Now),
			//nolint:gosec // int overflow
			NotCacheable(func(i int) int32 { return int32(i) }),
		),
		func(_ MyType1, _ MyType3) {},
		// CallsInner(func(i func()) { i() }),
		Memoize(func(i int32) int32 { return i }),
		OverridesError(func(_ func()) error { return nil }),
		MustConsume(func(i int32) int64 { return int64(i) }),
		ConsumptionOptional(func(i int64) float64 { return float64(i) }),
		func(_ MyType5) error { return nil },
		NonFinal(func() {}),
	)
	require.Error(t, err, "mess from the above")
	detailed := DetailedError(err)
	require.NotEqual(t, err.Error(), detailed, "detailed should have more")
	t.Log("detailed error", detailed)

	index := strings.Index(detailed, "func TestRegression")
	require.NotEqual(t, -1, index, "contains 'func TestRegression'")
	detailed = detailed[index:]

	for _, word := range strings.Split("Desired Shun Required Cacheable MustCache Cluster Memoize ShadowingAllowed\\[error\\] MustConsume ConsumptionOptional NonFinal", " ") {
		re := regexp.MustCompile(fmt.Sprintf(`\b%s\(` /*)*/, word))
		if !re.MatchString(detailed) {
			t.Errorf("did not find %s( in reproduce output", word) // )
		}
	}
}
