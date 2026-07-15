package metrics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func avgBefore(t *testing.T, rt *RateTracker[int64, int], excLimit int64) (float64, bool) {
	t.Helper()
	opt, err := rt.AverageBefore(excLimit)
	require.NoError(t, err)
	return opt.Get()
}

func TestRateTracker_EmptyAverage(t *testing.T) {
	rt := NewRateTracker[int64, int](time.Minute)
	_, ok := avgBefore(t, rt, 0)
	assert.False(t, ok, "AverageBefore() should be empty before any Set()")
}

func TestRateTracker_OneKey(t *testing.T) {
	rt := NewRateTracker[int64, int](time.Minute)
	require.NoError(t, rt.Set(1, 10))
	avg, ok := avgBefore(t, rt, 2)
	require.True(t, ok)
	assert.Equal(t, 10.0, avg)
}

func TestRateTracker_TwoConsecutiveKeys(t *testing.T) {
	rt := NewRateTracker[int64, int](time.Minute)
	require.NoError(t, rt.Set(1, 10))
	require.NoError(t, rt.Set(2, 20))

	avg, ok := avgBefore(t, rt, 3)
	require.True(t, ok)
	assert.Equal(t, 15.0, avg)
}

func TestRateTracker_SameKey(t *testing.T) {
	rt := NewRateTracker[int64, int](time.Minute)
	require.NoError(t, rt.Set(1, 3))
	require.NoError(t, rt.Set(1, 7))

	avg, ok := avgBefore(t, rt, 2)
	require.True(t, ok)
	// Set() replaces rather than accumulates; only one key so span=1.
	assert.Equal(t, 7.0, avg, "repeated Set() for the same key should replace, not accumulate")
}

func TestRateTracker_Gap(t *testing.T) {
	rt := NewRateTracker[int64, int](time.Minute)
	require.NoError(t, rt.Set(3, 5))
	require.NoError(t, rt.Set(5, 5))

	avg, ok := avgBefore(t, rt, 6)
	require.True(t, ok)
	// key 4 is a gap (count=0); span = 6-3 = 3; average = (5+5)/3.
	assert.InDelta(t, 10.0/3.0, avg, 1e-9)
}

func TestRateTracker_GapNoConsecutiveKeys(t *testing.T) {
	rt := NewRateTracker[int64, int](time.Minute)
	require.NoError(t, rt.Set(1, 12))
	require.NoError(t, rt.Set(4, 6))

	avg, ok := avgBefore(t, rt, 6)
	require.True(t, ok)
	// keys 2,3,5 are gaps; span = 6-1 = 5; average = (12+6)/5 = 3.6.
	assert.Equal(t, 3.6, avg)
}

func TestRateTracker_GapExceedsWindow(t *testing.T) {
	// Window is 3 seconds. A gap larger than the window must not let an old
	// entry stretch the span beyond the window size.
	rt := NewRateTracker[int64, int](3 * time.Second)
	require.NoError(t, rt.Set(1, 50)) // will fall outside the window once key 5 arrives
	require.NoError(t, rt.Set(5, 10))

	avg, ok := avgBefore(t, rt, 6)
	require.True(t, ok)
	// effectiveOldest = 6-3 = 3; key 1 < 3 is excluded.
	// Only key 5 (count=10) remains; span = 6-5 = 1.
	assert.Equal(t, 10.0, avg)
}

func TestRateTracker_PrecedingKeyErrors(t *testing.T) {
	rt := NewRateTracker[int64, int](time.Minute)
	require.NoError(t, rt.Set(5, 1))
	require.Error(t, rt.Set(4, 1), "Set() with a key before the last should return an error")
}

func TestRateTracker_AverageBeforeErrors(t *testing.T) {
	rt := NewRateTracker[int64, int](time.Minute)
	require.NoError(t, rt.Set(5, 1))
	_, err := rt.AverageBefore(5)
	assert.Error(t, err, "AverageBefore() with excLimit == newestKey should error")
	_, err = rt.AverageBefore(4)
	assert.Error(t, err, "AverageBefore() with excLimit < newestKey should error")
}

func TestRateTracker_WindowEviction(t *testing.T) {
	// Window is 3 seconds; keys 1–3 fill the window, key 4 evicts key 1.
	rt := NewRateTracker[int64, int](3 * time.Second)
	require.NoError(t, rt.Set(1, 100))
	require.NoError(t, rt.Set(2, 10))
	require.NoError(t, rt.Set(3, 10))
	require.NoError(t, rt.Set(4, 10))

	avg, ok := avgBefore(t, rt, 5)
	require.True(t, ok)
	// Ring holds keys 2,3,4; span = 5-2 = 3; average = (10+10+10)/3 = 10.
	assert.Equal(t, 10.0, avg)
}
