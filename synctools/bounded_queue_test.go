package synctools

import (
	"cmp"
	"context"
	"math/rand/v2"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type boundedQueueTestSuite struct {
	suite.Suite
}

func TestBoundedQueueTestSuite(t *testing.T) {
	suite.Run(t, &boundedQueueTestSuite{})
}

func (s *boundedQueueTestSuite) TestBasicSendReceive() {
	out, in, _ := NewBoundedQueue(s.T().Context(), 10, 1000, func(int) int64 { return 1 })

	// Send a few items
	for i := range 5 {
		in <- i
	}
	close(in)

	// Receive them back
	for i := range 5 {
		val := <-out
		s.Assert().Equal(i, val)
	}

	// Ensure the worker terminates and closes the output channel.
	_, ok := <-out
	s.Assert().False(ok)
}

func (s *boundedQueueTestSuite) TestCountLimitEnforced() {
	maxCount := 3
	out, in, _ := NewBoundedQueue(s.T().Context(), maxCount, 10000, func(int) int64 { return 1 })

	// Send maxCount + 1 items in a goroutine to avoid deadlock
	go func() {
		for i := range maxCount + 1 {
			in <- i
		}
		close(in)
	}()

	// When buffer hits limit and we send another, the first should be drained
	// Collect all outputs to verify they come in order
	items := lo.ChannelToSlice(out)

	s.Assert().Len(items, maxCount+1)
	// Verify items came in order
	for i := range items {
		s.Assert().Equal(i, items[i])
	}
}

func (s *boundedQueueTestSuite) TestMemoryLimitEnforced() {
	// maxCount is high (won't trigger); only memory limit should block.
	// Each item costs <= 20 bytes, maxMem=60. That max is a soft limit: we can
	// exceed it, but not by more than 1 item’s size. So the channel should
	// block once we hit 60 bytes, but the blocked send will push us to at most
	// 80 bytes before we drain back down.
	//
	// A concurrent observer tracks peak BufferedBytes.
	// With enforcement (soft limit): peak ≤ maxMem+maxItemSize = 80.
	// Without enforcement: peak could reach 100 × 20 = 2000.
	const maxMem = 60
	const maxItemSize = 20
	out, in, stats := NewBoundedQueue(
		s.T().Context(),
		100,
		maxMem,
		func(int) int64 { return rand.Int64N(maxItemSize) },
	)

	// Observer: track peak BufferedBytes.
	peakBytes := NewAtomicMax(int64(0), cmp.Compare)
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			default:
				peakBytes.Update(stats().BufferedBytes)
				runtime.Gosched()
			}
		}
	}()

	// Producer: send 100 items.
	go func() {
		for range 100 {
			in <- 0
		}
		close(in)
	}()

	received := consumeItems(out)
	close(done)

	s.Assert().Equal(100, received)

	// Since maxMem is a “soft” limit it’s possible to exceed it, but that will
	// never be by more than 1 item’s size.
	s.Assert().LessOrEqual(
		peakBytes.Get(),
		int64(maxMem+maxItemSize),
		"peak buffered bytes should not exceed maxMem by “much”",
	)
}

func (s *boundedQueueTestSuite) TestInputChannelClosed() {
	out, in, _ := NewBoundedQueue(s.T().Context(), 10, 1000, func(int) int64 { return 1 })

	// Send some items and close
	in <- 1
	in <- 2
	in <- 3
	close(in)

	// All items should be received
	items := lo.ChannelToSlice(out)

	s.Assert().Equal([]int{1, 2, 3}, items)
}

func (s *boundedQueueTestSuite) TestOversizedItem() {
	// A single item larger than maxMem should still pass through.
	// The memory limit is temporarily exceeded, then restored after drain.
	out, in, stats := NewBoundedQueue(
		s.T().Context(),
		10,
		50,
		func(i int) int64 { return int64(i) },
	)

	go func() {
		in <- 200
		in <- 200
		in <- 200
		close(in)
	}()

	items := lo.ChannelToSlice(out)

	s.Assert().Equal([]int{200, 200, 200}, items)

	snap := stats()
	s.Assert().Zero(snap.BufferedItems)
	s.Assert().Equal(int64(0), snap.BufferedBytes)
}

func (s *boundedQueueTestSuite) TestLastItemExceedsMemoryLimit() {
	t := s.T()

	// The total-size limit is soft: an item that pushes the total over maxMem
	// is still accepted, then the worker drains before accepting more.
	// Here maxMem=50, and we send 30+30=60 which exceeds the limit.
	// Both items should pass through, and stats should return to zero.
	out, in, stats := NewBoundedQueue(
		s.T().Context(),
		100,
		50,
		func(i int) int64 { return int64(i) },
	)

	sendBlocked := make(chan struct{})
	go func() {
		defer close(in)

		in <- 30 // curMem=30, under limit
		in <- 30 // curMem=60, over limit — allowed as the last item

		select {
		case in <- 30:
			require.Fail(t, "extra send", "must block once limit is met/exceeded")
		default:
		}

		close(sendBlocked)

		in <- 25 // curMem=55, over limit - allowed as the last item
	}()

	<-sendBlocked
	afterBlocked := pollStats(
		s.T(),
		stats,
		func(st BoundedQueueStats) bool { return st.BufferedItems == 2 },
	)
	s.Assert().
		Equal(2, afterBlocked.BufferedItems, "should have 2 items buffered after blocked send")
	s.Assert().
		Equal(int64(60), afterBlocked.BufferedBytes, "should have 60 bytes buffered after blocked send")

	items := lo.ChannelToSlice(out)

	s.Assert().Equal([]int{30, 30, 25}, items)

	snap := stats()
	s.Assert().Zero(snap.BufferedItems)
	s.Assert().Equal(int64(0), snap.BufferedBytes)
}

func (s *boundedQueueTestSuite) TestLargeMemoryItems() {
	maxMem := int64(100)
	out, in, _ := NewBoundedQueue(
		s.T().Context(),
		100,
		maxMem,
		func(b []byte) int64 { return int64(len(b)) },
	)

	go func() {
		// Send a large item
		largeItem := make([]byte, 50)
		in <- largeItem

		// Send another that pushes over limit
		in <- make([]byte, 60)
		close(in)
	}()

	// First item should come out
	val := <-out
	s.Assert().Len(val, 50)

	// Get the second
	val2 := <-out
	s.Assert().Len(val2, 60)

	// Channel should close
	_, ok := <-out
	s.Assert().False(ok)
}

func (s *boundedQueueTestSuite) TestManySmallItems() {
	out, in, _ := NewBoundedQueue(s.T().Context(), 5, 10000, func(int) int64 { return 1 })

	// Send 100 items through the channel
	go func() {
		for i := range 100 {
			in <- i
		}
		close(in)
	}()

	// Receive all items in order
	received := 0
	for item := range out {
		s.Assert().Equal(received, item)
		received++
	}

	s.Assert().Equal(100, received)
}

func (s *boundedQueueTestSuite) TestEmptyBuffer() {
	out, in, _ := NewBoundedQueue(s.T().Context(), 10, 1000, func(int) int64 { return 1 })

	// Send and receive immediately
	in <- 42
	val := <-out
	s.Assert().Equal(42, val)

	// Close channel
	close(in)

	// Output should close
	_, ok := <-out
	s.Assert().False(ok)
}

func (s *boundedQueueTestSuite) TestMemoryAndCountLimitsTogether() {
	// Limit to 3 items and 50 bytes
	out, in, _ := NewBoundedQueue(s.T().Context(), 3, 50, func(i int) int64 { return int64(i) })

	go func() {
		// Send items: 5, 10, 15 (total 30 bytes, 3 items)
		in <- 5
		in <- 10
		in <- 15

		// Send 20: total would be 50 (at limit)
		in <- 20

		// Should be drained now since count=4 > 3
		// Send 5 more
		in <- 5
		close(in)
	}()

	// Collect all items
	items := lo.ChannelToSlice(out)

	// All 5 items should come through
	s.Assert().Len(items, 5)
}

func (s *boundedQueueTestSuite) TestContextCancellation() {
	// Once the context is canceled the output channel should close immediately.
	// There’s a race here, though: the context won’t always cancel before the
	// worker starts sending things. To accommodate that, we repeat the test
	// until we see an incomplete output (fewer than 5 items), which indicates
	// that the context cancellation correctly prevented some sends. If we see
	// all 5 items, that means the context cancellation happened after all
	// sends, which is a valid but uninteresting outcome for this test, so we
	// retry until we see the interesting case.
	assert.Eventually(
		s.T(),
		func() bool {
			ctx, cancel := context.WithCancel(s.T().Context())
			out, in, _ := NewBoundedQueue(ctx, 10, 1000, func(int) int64 { return 1 })

			// Send some items.
			for i := range 5 {
				in <- i
			}
			close(in)

			// Cancel the context before reading.
			cancel()

			// The output channel should close after processing buffered items
			items := lo.ChannelToSlice(out)

			return len(items) < 5
		},
		time.Second,
		time.Millisecond,
		"output channel should close prematurely on context cancellation",
	)
}

func (s *boundedQueueTestSuite) TestZeroItemEdgeCases() {
	out, in, _ := NewBoundedQueue(s.T().Context(), 10, 1000, func(int) int64 {
		// Return 0 for size
		return 0
	})

	// With zero-size items, count limit is enforced (not memory)
	// Send in goroutine while receiving in main
	go func() {
		for i := range 100 {
			in <- i
		}
		close(in)
	}()

	// Should receive all items
	received := 0
	for range out {
		received++
	}

	s.Assert().Equal(100, received)
}

func (s *boundedQueueTestSuite) TestConcurrentStatsReads() {
	// Verify stats function is safe for concurrent reads while channel operates.
	// This tests atomic-safe access to both ringbuf.Len() and curMem.
	out, in, stats := NewBoundedQueue(
		s.T().Context(),
		10,
		1000,
		func(i int) int64 { return int64(i) },
	)

	done := make(chan struct{})
	statsCounter := startStatsReaders(stats, done)
	produceItems(in)
	received := consumeItems(out)
	s.Assert().Equal(100, received)

	close(done)
	s.Assert().
		Positive(statsCounter.Load(), "stats reader should have executed and seen valid data")
}

// startStatsReaders launches multiple goroutines that concurrently read stats.
func startStatsReaders(stats func() BoundedQueueStats, done chan struct{}) *atomic.Int64 {
	statsCounter := &atomic.Int64{}
	for range 5 {
		go statsReaderLoop(stats, done, statsCounter)
	}
	return statsCounter
}

// statsReaderLoop continuously reads stats until done is closed.
func statsReaderLoop(stats func() BoundedQueueStats, done chan struct{}, counter *atomic.Int64) {
	for {
		select {
		case <-done:
			return
		default:
			snap := stats()
			if isValidSnapshot(snap) {
				counter.Add(1)
			}
			runtime.Gosched()
		}
	}
}

// isValidSnapshot verifies that a stats snapshot is within expected bounds.
// IMPORTANT: This is valid ONLY where no over-limit buffering can happen.
func isValidSnapshot(snap BoundedQueueStats) bool {
	return snap.BufferedItems >= 0 && snap.BufferedItems <= snap.MaxItems &&
		snap.BufferedBytes >= 0 && snap.BufferedBytes <= snap.MaxBytes
}

// produceItems sends items 1..100 to the channel then closes it.
func produceItems(in chan<- int) {
	go func() {
		for i := range 100 {
			in <- 1 + i
		}
		close(in)
	}()
}

// consumeItems receives all items from the channel and returns the count.
func consumeItems(out <-chan int) int {
	received := 0
	for range out {
		received++
	}
	return received
}

func (s *boundedQueueTestSuite) TestStatsAccuracy() {
	// Verify stats reflect actual buffer state at snapshot time.
	out, in, stats := NewBoundedQueue(
		s.T().Context(),
		5,
		1000,
		func(i int) int64 { return int64(i) },
	)

	// Send 3 items: size 1, 2, 3 = 6 total.
	// We must poll for the stats because unbuffered channel sends complete when
	// the receiver starts receiving, not when it finishes processing the item.
	// Using a goroutine to send ensures the worker goroutine has time to process.
	sentDone := make(chan struct{})
	go func() {
		in <- 1
		in <- 2
		in <- 3
		close(sentDone)
	}()

	snap := pollStats(
		s.T(),
		stats,
		func(st BoundedQueueStats) bool { return st.BufferedItems == 3 },
	)
	s.Assert().Equal(3, snap.BufferedItems)
	s.Assert().Equal(int64(6), snap.BufferedBytes)
	s.Assert().Equal(5, snap.MaxItems)
	s.Assert().Equal(int64(1000), snap.MaxBytes)

	<-sentDone

	// Close and drain remaining
	close(in)
	for range out {
	}

	snap = stats()
	s.Assert().Zero(snap.BufferedItems)
	s.Assert().Equal(int64(0), snap.BufferedBytes)
}

func (s *boundedQueueTestSuite) TestSizeComputedOncePerItem() {
	// Verify that size is computed once on receipt and reused on drain.
	// Count size() invocations directly so the test deterministically fails if
	// size() is called more than once per item.
	var sizeCalls atomic.Int64
	sizeFunc := func(i int) int64 {
		sizeCalls.Add(1)
		return int64(i + 1)
	}

	out, in, stats := NewBoundedQueue(s.T().Context(), 10, 1000, sizeFunc)

	// Send items
	go func() {
		for i := range 10 {
			in <- i
		}
		close(in)
	}()

	// Receive all items
	received := 0
	for range out {
		received++
	}

	s.Assert().Equal(10, received)
	s.Assert().EqualValues(10, sizeCalls.Load(), "size() should be called exactly once per item")

	// Verify final stats: no items buffered, no bytes buffered
	snap := stats()
	s.Assert().Equal(0, snap.BufferedItems, "all items should be drained")
	s.Assert().
		Equal(int64(0), snap.BufferedBytes, "all bytes should be accounted for (curMem should be 0)")
}

// pollStats polls until the given predicate is true, checking stats repeatedly.
func pollStats(
	t *testing.T,
	stats func() BoundedQueueStats,
	predicate func(BoundedQueueStats) bool,
) BoundedQueueStats {
	var snap BoundedQueueStats

	require.Eventually(
		t,
		func() bool {
			snap = stats()
			return predicate(snap)
		},
		time.Minute,
		time.Millisecond,
	)

	return snap
}

func (s *boundedQueueTestSuite) TestCountBoundIsInclusive() {
	// Verify that maxCount limit is inclusive: we can buffer exactly maxCount items without draining.
	out, in, stats := NewBoundedQueue(s.T().Context(), 3, 1000, func(int) int64 { return 1 })

	sentDone := make(chan struct{})
	go func() {
		// Send exactly 3 items; they should all stay buffered (not drained)
		in <- 1
		in <- 2
		in <- 3
		// Signal that sending is complete; goroutine exits immediately after
		close(sentDone)
	}()

	// Poll until items are buffered or we know sending is done
	snap := pollStats(
		s.T(),
		stats,
		func(st BoundedQueueStats) bool { return st.BufferedItems == 3 },
	)
	s.Assert().Equal(3, snap.BufferedItems, "should be able to buffer exactly maxCount items")

	// Wait for sending to complete, then close input and drain
	<-sentDone
	close(in)
	for range out {
	}

	snap = stats()
	s.Assert().Zero(snap.BufferedItems)
}

func (s *boundedQueueTestSuite) TestMemoryBelowLimitStaysBuffered() {
	// Items totaling strictly less than maxMem should all stay buffered.
	// 10 + 20 + 29 = 59 < maxMem = 60: no forced drain.
	out, in, stats := NewBoundedQueue(
		s.T().Context(),
		100,
		60,
		func(i int) int64 { return int64(i) },
	)

	sentDone := make(chan struct{})
	go func() {
		in <- 10
		in <- 20
		in <- 29
		close(sentDone)
	}()

	snap := pollStats(
		s.T(),
		stats,
		func(st BoundedQueueStats) bool { return st.BufferedItems == 3 },
	)
	s.Assert().Equal(3, snap.BufferedItems, "should have 3 items")
	s.Assert().Equal(int64(59), snap.BufferedBytes, "items below maxMem should stay buffered")

	<-sentDone
	close(in)
	for range out {
	}

	snap = stats()
	s.Assert().Equal(int64(0), snap.BufferedBytes)
}
