package synctools

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type boundedChanTestSuite struct {
	suite.Suite
}

func TestBoundedChanTestSuite(t *testing.T) {
	suite.Run(t, &boundedChanTestSuite{})
}

func (s *boundedChanTestSuite) TestBasicSendReceive() {
	out, in := NewBoundedChan(10, 1000, func(i int) int64 { return 1 })

	// Send a few items
	for i := range 5 {
		in <- i
	}

	// Receive them back
	for i := range 5 {
		val := <-out
		s.Equal(i, val)
	}
}

func (s *boundedChanTestSuite) TestCountLimitEnforced() {
	maxCount := int64(3)
	out, in := NewBoundedChan(maxCount, 10000, func(i int) int64 { return 1 })

	// Send maxCount + 1 items in a goroutine to avoid deadlock
	go func() {
		for i := range maxCount + 1 {
			in <- int(i)
		}
		close(in)
	}()

	// When buffer hits limit and we send another, the first should be drained
	// Collect all outputs to verify they come in order
	items := []int{}
	for item := range out {
		items = append(items, item)
	}

	s.Equal(int(maxCount)+1, len(items))
	// Verify items came in order
	for i := 0; i < len(items); i++ {
		s.Equal(i, items[i])
	}
}

func (s *boundedChanTestSuite) TestMemoryLimitEnforced() {
	maxMem := int64(100)
	out, in := NewBoundedChan(1000, maxMem, func(i int) int64 { return int64(i) })

	// Send items in goroutine to avoid deadlock
	go func() {
		// Send items that total > maxMem
		// The sum is 85, which is under 100, so all should be buffered
		// Then send one that pushes us over (10, 20, 30, 25, 20 = 105)
		itemsToSend := []int{10, 20, 30, 25, 20}
		for _, item := range itemsToSend {
			in <- item
		}
		close(in)
	}()

	// Collect items and verify first one comes out
	items := []int{}
	for item := range out {
		items = append(items, item)
	}

	// All items should come through in order
	s.Equal(5, len(items))
	s.Equal(10, items[0])
}

func (s *boundedChanTestSuite) TestInputChannelClosed() {
	out, in := NewBoundedChan(10, 1000, func(i int) int64 { return 1 })

	// Send some items and close
	in <- 1
	in <- 2
	in <- 3
	close(in)

	// All items should be received
	items := []int{}
	for item := range out {
		items = append(items, item)
	}

	s.Equal([]int{1, 2, 3}, items)
}

func (s *boundedChanTestSuite) TestLargeMemoryItems() {
	maxMem := int64(100)
	out, in := NewBoundedChan(100, maxMem, func(b []byte) int64 { return int64(len(b)) })

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
	s.Equal(50, len(val))

	// Get the second
	val2 := <-out
	s.Equal(60, len(val2))

	// Channel should close
	_, ok := <-out
	s.False(ok)
}

func (s *boundedChanTestSuite) TestManySmallItems() {
	out, in := NewBoundedChan(5, 10000, func(i int) int64 { return 1 })

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
		s.Equal(received, item)
		received++
	}

	s.Equal(100, received)
}

func (s *boundedChanTestSuite) TestEmptyBuffer() {
	out, in := NewBoundedChan(10, 1000, func(i int) int64 { return 1 })

	// Send and receive immediately
	in <- 42
	val := <-out
	s.Equal(42, val)

	// Close channel
	close(in)

	// Output should close
	_, ok := <-out
	s.False(ok)
}

func (s *boundedChanTestSuite) TestMemoryAndCountLimitsTogether() {
	// Limit to 3 items and 50 bytes
	out, in := NewBoundedChan(3, 50, func(i int) int64 { return int64(i) })

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
	items := []int{}
	for item := range out {
		items = append(items, item)
	}

	// All 5 items should come through
	s.Equal(5, len(items))
}

func (s *boundedChanTestSuite) TestZeroItemEdgeCases() {
	out, in := NewBoundedChan(10, 1000, func(i int) int64 {
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

	s.Equal(100, received)
}
