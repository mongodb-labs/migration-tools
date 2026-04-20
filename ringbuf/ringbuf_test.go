package ringbuf

import (
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ringbufTestSuite struct {
	suite.Suite
}

func TestRingBufTestSuite(t *testing.T) {
	suite.Run(t, &ringbufTestSuite{})
}

func (s *ringbufTestSuite) TestBasicPushPop() {
	r := New[int](3)
	s.Assert().Equal(0, r.Len())
	s.Assert().Equal(3, r.Cap())

	r.Push(10)
	s.Assert().Equal(1, r.Len())
	s.Assert().Equal(10, r.Peek())

	r.Push(20)
	r.Push(30)
	s.Assert().Equal(3, r.Len())

	s.Assert().Equal(10, r.Peek())
	r.Pop()
	s.Assert().Equal(2, r.Len())

	s.Assert().Equal(20, r.Peek())
	r.Pop()
	s.Assert().Equal(1, r.Len())

	s.Assert().Equal(30, r.Peek())
	r.Pop()
	s.Assert().Equal(0, r.Len())
}

func (s *ringbufTestSuite) TestWrapAround() {
	r := New[int](3)

	// Fill the buffer: [10, 20, 30]
	r.Push(10)
	r.Push(20)
	r.Push(30)
	s.Assert().Equal(3, r.Len())

	// Pop one: [_, 20, 30], head advances
	r.Pop()
	s.Assert().Equal(2, r.Len())

	// Push one, wrapping: [40, 20, 30], tail wraps around
	r.Push(40)
	s.Assert().Equal(3, r.Len())

	// Items should come out in order
	s.Assert().Equal(20, r.Peek())
	r.Pop()
	s.Assert().Equal(30, r.Peek())
	r.Pop()
	s.Assert().Equal(40, r.Peek())
	r.Pop()
	s.Assert().Equal(0, r.Len())
}

func (s *ringbufTestSuite) TestMultipleWraps() {
	r := New[int](2)

	// Cycle: [1, 2] -> pop -> push 3 -> pop -> push 4, etc.
	r.Push(1)
	r.Push(2)
	s.Assert().Equal(2, r.Len())

	r.Pop() // head wraps from 0->1
	s.Assert().Equal(1, r.Len())

	r.Push(3) // tail wraps from 1->0
	s.Assert().Equal(2, r.Len())
	s.Assert().Equal(2, r.Peek())

	r.Pop() // head wraps from 1->0
	r.Pop() // head wraps from 0->1
	s.Assert().Equal(0, r.Len())
}

func (s *ringbufTestSuite) TestEmptyPanicPeek() {
	r := New[int](1)
	s.Assert().PanicsWithValue("assertion failed: peek needs nonempty buffer", func() {
		r.Peek()
	})
}

func (s *ringbufTestSuite) TestEmptyPanicPop() {
	r := New[int](1)
	s.Assert().PanicsWithValue("assertion failed: pop needs nonempty buffer", func() {
		r.Pop()
	})
}

func (s *ringbufTestSuite) TestFullPanicPush() {
	r := New[int](2)
	r.Push(1)
	r.Push(2)
	s.Assert().PanicsWithValue("assertion failed: buffer must be non-full", func() {
		r.Push(3)
	})
}

func (s *ringbufTestSuite) TestPointerTypes() {
	type obj struct {
		val int
	}

	r := New[*obj](2)
	obj1 := &obj{val: 100}
	obj2 := &obj{val: 200}

	r.Push(obj1)
	r.Push(obj2)

	s.Assert().Equal(obj1.val, r.Peek().val)
	r.Pop()

	s.Assert().Equal(obj2.val, r.Peek().val)
	r.Pop()

	s.Assert().Equal(0, r.Len())
}

func (s *ringbufTestSuite) TestZeroValuesReleased() {
	// After Pop(), the slot should be zeroed so that pointers are released for GC.
	// Since this test is in package ringbuf, it can directly inspect the internal buffer.
	r := New[*int](1)
	val := new(int)
	*val = 42

	r.Push(val)
	s.Assert().Equal(val, r.Peek())

	r.Pop()
	s.Assert().Equal(0, r.Len())

	// The only slot in a capacity-1 buffer must be reset to the zero value after Pop().
	s.Assert().Nil(r.buf[0], "popped slot should be zeroed to release pointers for GC")
}

func (s *ringbufTestSuite) TestConcurrentLenReads() {
	// Verify Len() is safe for concurrent reads while single-threaded Push/Pop occurs.
	// This tests the atomic.Int64 safety of the count field.
	r := New[int](100)

	// Start goroutines constantly reading Len()
	done := make(chan struct{})
	for range 10 {
		go func() {
			for {
				select {
				case <-done:
					return
				default:
					_ = r.Len() // Read concurrently, should not race
					runtime.Gosched()
				}
			}
		}()
	}

	// Main thread pushes and pops
	for i := range 100 {
		r.Push(i)
		s.Assert().Equal(i+1, r.Len())
	}
	for i := range 100 {
		r.Pop()
		s.Assert().Equal(100-i-1, r.Len())
	}

	close(done)
}

func (s *ringbufTestSuite) TestConcurrentCapReads() {
	// Verify Cap() is safe for concurrent reads (it's trivial but let's verify pattern).
	r := New[int](42)

	done := make(chan struct{})
	defer close(done)

	var capCounter atomic.Int32
	for range 5 {
		go func() {
			for {
				select {
				case <-done:
					return
				default:
					if r.Cap() == 42 {
						capCounter.Add(1)
					}
					runtime.Gosched()
				}
			}
		}()
	}

	// Do some operations
	for i := range 42 {
		r.Push(i)
	}

	require.Eventually(
		s.T(),
		func() bool {
			return capCounter.Load() > 0
		},
		time.Minute,
		time.Millisecond,
	)

	// Verify concurrent reads actually executed
	s.Assert().Positive(capCounter.Load(), "cap reader should have executed")
}
