package metrics

import (
	"cmp"
	"fmt"
	"time"

	"github.com/mongodb-labs/migration-tools/metrics"
	"github.com/mongodb-labs/migration-tools/option"
)

// ChangeStreamMetrics tracks metrics for a change stream. It is *not*
// thread-safe, so use it only from a single goroutine.
type ChangeStreamMetrics struct {
	lastClusterTimeT    uint32
	curClusterTimeCount int
	lastWallSecond      int64
	curWallSecondCount  int

	readTracker         *metrics.RateTracker[int64, int]
	clusterWriteTracker *metrics.RateTracker[uint32, int]
}

// NewChangeStreamMetrics creates a new ChangeStreamMetrics that computes
// metrics over the given duration. The duration must be at least 1 second.
func NewChangeStreamMetrics(
	duration time.Duration,
) *ChangeStreamMetrics {
	return &ChangeStreamMetrics{
		readTracker:         metrics.NewRateTracker[int64, int](duration),
		clusterWriteTracker: metrics.NewRateTracker[uint32, int](duration),
	}
}

func (m *ChangeStreamMetrics) Add(clusterTimeT uint32) error {
	if m.lastClusterTimeT == 0 {
		m.lastClusterTimeT = clusterTimeT
	}

	switch cmp.Compare(clusterTimeT, m.lastClusterTimeT) {
	case 0:
		m.curClusterTimeCount++
	case 1:
		if err := m.clusterWriteTracker.Add(m.lastClusterTimeT, m.curClusterTimeCount); err != nil {
			return err
		}
		m.lastClusterTimeT = clusterTimeT
		m.curClusterTimeCount = 1
	default:
		return fmt.Errorf(
			"clusterTime.T (%d) precedes most recent clusterTime.T (%d)",
			clusterTimeT,
			m.lastClusterTimeT,
		)
	}

	wallSecond := time.Now().Unix()
	if m.lastWallSecond == 0 {
		m.lastWallSecond = wallSecond
	}

	switch cmp.Compare(wallSecond, m.lastWallSecond) {
	case 0:
		m.curWallSecondCount++
	case 1:
		if err := m.readTracker.Add(m.lastWallSecond, m.curWallSecondCount); err != nil {
			return err
		}
		m.lastWallSecond = wallSecond
		m.curWallSecondCount = 1
	default:
		return fmt.Errorf(
			"wallSecond (%d) precedes most recent wallSecond (%d)",
			wallSecond,
			m.lastWallSecond,
		)
	}

	return nil
}

func (m *ChangeStreamMetrics) EventsReadPerSecond() option.Option[float64] {
	return m.readTracker.Average()
}

func (m *ChangeStreamMetrics) ClusterEventsPerSecond() option.Option[float64] {
	return m.clusterWriteTracker.Average()
}
