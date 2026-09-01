package metrics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// addN calls m.Add(clusterTimeT) n times and requires no error.
func addN(t *testing.T, m *ChangeStreamMetrics[int64], clusterTimeT int64, n int) {
	t.Helper()
	for range n {
		require.NoError(t, m.Add(clusterTimeT))
	}
}

func newCSM(t *testing.T) *ChangeStreamMetrics[int64] {
	t.Helper()
	return NewChangeStreamMetrics[int64](time.Minute)
}

func TestChangeStreamMetrics_EmptyAverage(t *testing.T) {
	m := newCSM(t)
	assert.False(t, m.ClusterEventsPerSecond().IsSome())
}

func TestChangeStreamMetrics_OneClusterTime(t *testing.T) {
	m := newCSM(t)
	addN(t, m, 1, 5)
	// No bucket is ever completed.
	assert.False(t, m.ClusterEventsPerSecond().IsSome())
}

func TestChangeStreamMetrics_TwoClusterTimes(t *testing.T) {
	m := newCSM(t)
	addN(t, m, 1, 3)
	addN(t, m, 2, 5)
	// The first completed bucket (key 1) is skipped; key 2 is still in progress.
	assert.False(t, m.ClusterEventsPerSecond().IsSome())
}

func TestChangeStreamMetrics_ThreeClusterTimes(t *testing.T) {
	m := newCSM(t)
	addN(t, m, 1, 3) // skipped (first completed bucket)
	addN(t, m, 2, 5)
	addN(t, m, 3, 1) // in progress

	avg, ok := m.ClusterEventsPerSecond().Get()
	require.True(t, ok)
	// Only key 2 (count=5) is recorded; span=1.
	assert.Equal(t, 5.0, avg)
}

func TestChangeStreamMetrics_FourClusterTimes(t *testing.T) {
	m := newCSM(t)
	addN(t, m, 1, 3) // skipped
	addN(t, m, 2, 5)
	addN(t, m, 3, 7)
	addN(t, m, 4, 1) // in progress

	avg, ok := m.ClusterEventsPerSecond().Get()
	require.True(t, ok)
	// Keys 2 (count=5) and 3 (count=7) recorded; span = 3-2+1 = 2.
	assert.Equal(t, 6.0, avg)
}

func TestChangeStreamMetrics_Gap(t *testing.T) {
	m := newCSM(t)
	addN(t, m, 1, 3) // skipped
	addN(t, m, 2, 10)
	addN(t, m, 4, 10) // key 3 is a gap
	addN(t, m, 6, 1)  // in progress; key 5 is a gap

	avg, ok := m.ClusterEventsPerSecond().Get()
	require.True(t, ok)
	// Keys 2 (10) and 4 (10) recorded; keys 3 & 5 are gaps; span = 4.
	assert.Equal(t, 5.0, avg)
}

func TestChangeStreamMetrics_PrecedingKeyError(t *testing.T) {
	m := newCSM(t)
	require.NoError(t, m.Add(5))
	assert.Error(t, m.Add(4))
}

func TestChangeStreamMetrics_ZeroKeyError(t *testing.T) {
	m := newCSM(t)
	assert.Error(t, m.Add(0))
}
