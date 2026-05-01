package history

import (
	"runtime"
	"slices"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHistory(t *testing.T) {
	h := New[int](time.Hour)

	assert.Equal(t, 1, h.Add(234))
	assert.Equal(t, 2, h.Add(234))
	assert.Equal(t, 3, h.Add(345))

	got := h.Get()
	times, data := splitLogs(got)
	assert.True(
		t,
		slices.IsSortedFunc(times, time.Time.Compare),
		"times should be increasing",
	)
	assert.Equal(t, []int{234, 234, 345}, data, "data as expected")

	got[0].Datum = 999
	got = h.Get()
	_, data = splitLogs(got)
	assert.Equal(t, []int{234, 234, 345}, data, "slice is copied")
}

func TestHistoryTTL(t *testing.T) {
	h := New[int](time.Millisecond)

	assert.Equal(t, 1, h.Add(234))
	assert.Equal(t, 2, h.Add(234))
	assert.Equal(t, 3, h.Add(345))

	assert.Eventually(
		t,
		func() bool {
			return len(h.Get()) == 0
		},
		time.Minute,
		time.Millisecond,
		"history should expire its entries",
	)

	assert.Equal(t, 1, h.Add(234), "new record should be the first")
}

func splitLogs[T any](in []Log[T]) ([]time.Time, []T) {
	var times []time.Time
	var data []T

	for _, cur := range in {
		times = append(times, cur.At)
		data = append(data, cur.Datum)
	}

	return times, data
}

// TestAddAfterReapDoesNotAllocate verifies the steady-state invariant that
// motivates the in-place reap implementation: once the underlying slice has
// reached some capacity, an Add that follows a reap does not allocate. The
// reap is in-place (slices.Delete) so capacity is preserved, and the
// subsequent append fits in the existing capacity.
func TestAddAfterReapDoesNotAllocate(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const ttl = time.Minute
		h := New[int](ttl)

		// Prime the slice so the underlying array reaches a stable capacity.
		// Subsequent reap+append cycles then operate within that capacity.
		for i := range 64 {
			h.Add(i)
		}

		// Each iteration: advance past TTL so the previous iteration's entry
		// expires, then Add. The Add reaps in-place (no alloc) and appends
		// into existing capacity (no alloc).
		// Manual memstats measurement, since synctest interferes with
		// testing.AllocsPerRun's bookkeeping (it reports 0 even when there
		// are real allocations).
		//
		// The reap-then-append cycle should allocate ~0 per iteration. A
		// regression to a naive reslice (h.logs = h.logs[k:]) loses capacity
		// each time and allocates on every append (~1 per iteration after
		// growth amortization). We allow a small baseline for synctest
		// internals; well under the per-iteration count.
		var before, after runtime.MemStats
		const iterations = 100
		runtime.GC()
		runtime.ReadMemStats(&before)
		for range iterations {
			time.Sleep(2 * ttl)
			h.Add(0)
		}
		runtime.ReadMemStats(&after)
		allocs := after.Mallocs - before.Mallocs

		t.Logf("total mallocs over %d iterations = %d", iterations, allocs)
		assert.Less(t, int(allocs), iterations/10,
			"Add following reap should not allocate per iteration")
	})
}

func TestRatePer(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		history := New[int64](time.Minute)

		history.Add(123)

		time.Sleep(time.Second)

		perSec := RatePer(history.Get(), time.Second)
		assert.EqualValues(t, 123, perSec)

		perMin := RatePer(history.Get(), time.Minute)
		assert.EqualValues(t, 123*60, perMin)

		history.Add(123)

		perSec = RatePer(history.Get(), time.Second)
		assert.EqualValues(t, 246, perSec)

		perMin = RatePer(history.Get(), time.Minute)
		assert.EqualValues(t, 246*60, perMin)
	})
}
