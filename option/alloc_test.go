package option

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// allocFreeRuns is the number of iterations testing.AllocsPerRun executes.
// More iterations make the average more accurate.
const allocFreeRuns = 100

// largeStruct is intentionally larger than a machine word so that boxing it
// into an interface{} (e.g. via `args ...any`) forces a heap allocation.
type largeStruct struct {
	a, b, c, d int64
	s          string
}

func assertNoAllocs(t *testing.T, name string, fn func()) {
	t.Helper()
	avg := testing.AllocsPerRun(allocFreeRuns, fn)

	assert.Zero(t, avg, "%s: allocations", name)
}

func TestSomeAllocs(t *testing.T) {
	t.Run("int", func(t *testing.T) {
		assertNoAllocs(t, "Some[int]", func() {
			_ = Some(42)
		})
	})

	t.Run("string", func(t *testing.T) {
		s := "hello"
		assertNoAllocs(t, "Some[string]", func() {
			_ = Some(s)
		})
	})

	t.Run("bool", func(t *testing.T) {
		assertNoAllocs(t, "Some[bool]", func() {
			_ = Some(true)
		})
	})

	t.Run("largeStruct", func(t *testing.T) {
		v := largeStruct{a: 1, b: 2, c: 3, d: 4, s: "x"}
		assertNoAllocs(t, "Some[largeStruct]", func() {
			_ = Some(v)
		})
	})

	t.Run("pointer (non-nil)", func(t *testing.T) {
		x := 7
		p := &x
		assertNoAllocs(t, "Some[*int]", func() {
			_ = Some(p)
		})
	})

	t.Run("slice (non-nil)", func(t *testing.T) {
		s := []int{1, 2, 3}
		assertNoAllocs(t, "Some[[]int]", func() {
			_ = Some(s)
		})
	})
}

func TestNoneAllocs(t *testing.T) {
	assertNoAllocs(t, "None[int]", func() {
		_ = None[int]()
	})
	assertNoAllocs(t, "None[largeStruct]", func() {
		_ = None[largeStruct]()
	})
}

func TestFromPointerNoAllocs(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		assertNoAllocs(t, "FromPointer[int](nil)", func() {
			_ = FromPointer[int](nil)
		})
	})

	t.Run("non-nil", func(t *testing.T) {
		x := 42
		assertNoAllocs(t, "FromPointer[int](&x)", func() {
			_ = FromPointer(&x)
		})
	})

	t.Run("non-nil largeStruct", func(t *testing.T) {
		v := largeStruct{a: 1, b: 2, c: 3, d: 4, s: "x"}
		assertNoAllocs(t, "FromPointer[largeStruct](&v)", func() {
			_ = FromPointer(&v)
		})
	})
}

func TestOptionMethodsNoAllocs(t *testing.T) {
	some := Some(42)
	none := None[int]()

	assertNoAllocs(t, "Some.Get", func() {
		_, _ = some.Get()
	})
	assertNoAllocs(t, "None.Get", func() {
		_, _ = none.Get()
	})
	assertNoAllocs(t, "Some.IsSome", func() {
		_ = some.IsSome()
	})
	assertNoAllocs(t, "Some.IsNone", func() {
		_ = some.IsNone()
	})
	assertNoAllocs(t, "Some.OrZero", func() {
		_ = some.OrZero()
	})
	assertNoAllocs(t, "None.OrZero", func() {
		_ = none.OrZero()
	})
	assertNoAllocs(t, "Some.OrElse", func() {
		_ = some.OrElse(99)
	})
	assertNoAllocs(t, "None.OrElse", func() {
		_ = none.OrElse(99)
	})
	assertNoAllocs(t, "Some.MustGet", func() {
		_ = some.MustGet()
	})
	assertNoAllocs(t, "Some.MustGetf", func() {
		_ = some.MustGetf("this should never fail")
	})
}
