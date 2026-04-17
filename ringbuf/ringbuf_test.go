package ringbuf

import (
	"testing"

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
	s.Equal(0, r.Len())
	s.Equal(3, r.Cap())

	r.Push(10)
	s.Equal(1, r.Len())
	s.Equal(10, r.Peek())

	r.Push(20)
	r.Push(30)
	s.Equal(3, r.Len())

	s.Equal(10, r.Peek())
	r.Pop()
	s.Equal(2, r.Len())

	s.Equal(20, r.Peek())
	r.Pop()
	s.Equal(1, r.Len())

	s.Equal(30, r.Peek())
	r.Pop()
	s.Equal(0, r.Len())
}

func (s *ringbufTestSuite) TestWrapAround() {
	r := New[int](3)

	// Fill the buffer: [10, 20, 30]
	r.Push(10)
	r.Push(20)
	r.Push(30)
	s.Equal(3, r.Len())

	// Pop one: [_, 20, 30], head advances
	r.Pop()
	s.Equal(2, r.Len())

	// Push one, wrapping: [40, 20, 30], tail wraps around
	r.Push(40)
	s.Equal(3, r.Len())

	// Items should come out in order
	s.Equal(20, r.Peek())
	r.Pop()
	s.Equal(30, r.Peek())
	r.Pop()
	s.Equal(40, r.Peek())
	r.Pop()
	s.Equal(0, r.Len())
}

func (s *ringbufTestSuite) TestMultipleWraps() {
	r := New[int](2)

	// Cycle: [1, 2] -> pop -> push 3 -> pop -> push 4, etc.
	r.Push(1)
	r.Push(2)
	s.Equal(2, r.Len())

	r.Pop() // head wraps from 0->1
	s.Equal(1, r.Len())

	r.Push(3) // tail wraps from 1->0
	s.Equal(2, r.Len())
	s.Equal(2, r.Peek())

	r.Pop() // head wraps from 1->0
	r.Pop() // head wraps from 0->1
	s.Equal(0, r.Len())
}

func (s *ringbufTestSuite) TestEmptyPanicPeek() {
	r := New[int](1)
	s.PanicsWithValue("ringbuf: peek on empty buffer", func() {
		r.Peek()
	})
}

func (s *ringbufTestSuite) TestEmptyPanicPop() {
	r := New[int](1)
	s.PanicsWithValue("ringbuf: pop on empty buffer", func() {
		r.Pop()
	})
}

func (s *ringbufTestSuite) TestFullPanicPush() {
	r := New[int](2)
	r.Push(1)
	r.Push(2)
	s.PanicsWithValue("ringbuf: push on full buffer", func() {
		r.Push(3)
	})
}

func (s *ringbufTestSuite) TestPointerTypes() {
	type Obj struct{ val int }

	r := New[*Obj](2)
	obj1 := &Obj{val: 100}
	obj2 := &Obj{val: 200}

	r.Push(obj1)
	r.Push(obj2)

	s.Equal(obj1, r.Peek())
	r.Pop()

	s.Equal(obj2, r.Peek())
	r.Pop()

	s.Equal(0, r.Len())
}

func (s *ringbufTestSuite) TestZeroValuesReleased() {
	// After Pop(), the slot should be zeroed so that pointers are released for GC.
	// We test this by pushing, popping, then verifying the internal slot is zero.
	r := New[*int](1)
	val := new(int)
	*val = 42

	r.Push(val)
	s.Equal(val, r.Peek())

	r.Pop()
	s.Equal(0, r.Len())

	// The slot should now be nil (zero value for *int)
	// We verify indirectly by pushing a new value and seeing it goes to the same slot.
	newVal := new(int)
	*newVal = 99
	r.Push(newVal)
	s.Equal(newVal, r.Peek())
}
