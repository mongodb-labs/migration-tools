package synctools

import (
	"cmp"
	"runtime"
	"sync"
	"testing"

	"github.com/stretchr/testify/suite"
)

type atomicMaxTestSuite struct {
	suite.Suite
}

func TestAtomicMaxTestSuite(t *testing.T) {
	suite.Run(t, &atomicMaxTestSuite{})
}

func (s *atomicMaxTestSuite) TestInitialValue() {
	am := NewAtomicMax(42, cmp.Compare[int])
	s.Assert().Equal(42, am.Get())
}

func (s *atomicMaxTestSuite) TestUpdateHigherValue() {
	am := NewAtomicMax(0, cmp.Compare[int])
	prev := am.Update(10)
	s.Assert().Equal(0, prev, "should return previous max")
	s.Assert().Equal(10, am.Get())
}

func (s *atomicMaxTestSuite) TestUpdateLowerValueIgnored() {
	am := NewAtomicMax(50, cmp.Compare[int])
	prev := am.Update(25)
	s.Assert().Equal(50, prev, "should return current max when update rejected")
	s.Assert().Equal(50, am.Get())
}

func (s *atomicMaxTestSuite) TestUpdateEqualValueIgnored() {
	am := NewAtomicMax(50, cmp.Compare[int])
	prev := am.Update(50)
	s.Assert().Equal(50, prev, "should return current max when update rejected")
	s.Assert().Equal(50, am.Get())
}

func (s *atomicMaxTestSuite) TestUpdateReturnsPreviousMaxOnSuccessiveUpdates() {
	am := NewAtomicMax(0, cmp.Compare[int])
	s.Assert().Equal(0, am.Update(10))
	s.Assert().Equal(10, am.Update(20))
	s.Assert().Equal(20, am.Update(30))
	s.Assert().Equal(30, am.Update(15)) // rejected, but returns current max
	s.Assert().Equal(30, am.Update(30)) // equal, also returns current
	s.Assert().Equal(30, am.Get())
}

func (s *atomicMaxTestSuite) TestMultipleUpdatesKeepsMax() {
	am := NewAtomicMax(0, cmp.Compare[int])
	for _, v := range []int{5, 3, 8, 1, 9, 2, 7} {
		am.Update(v)
	}
	s.Assert().Equal(9, am.Get())
}

func (s *atomicMaxTestSuite) TestNegativeValues() {
	am := NewAtomicMax(-100, cmp.Compare[int])
	am.Update(-50)
	s.Assert().Equal(-50, am.Get())

	am.Update(-75)
	s.Assert().Equal(-50, am.Get())
}

func (s *atomicMaxTestSuite) TestStringMax() {
	am := NewAtomicMax("", cmp.Compare[string])
	am.Update("banana")
	am.Update("apple")
	am.Update("cherry")
	s.Assert().Equal("cherry", am.Get())
}

func (s *atomicMaxTestSuite) TestCustomComparer() {
	// Track the max by absolute value.
	absCmp := func(a, b int) int {
		absA, absB := a, b
		if absA < 0 {
			absA = -absA
		}
		if absB < 0 {
			absB = -absB
		}
		return cmp.Compare(absA, absB)
	}

	am := NewAtomicMax(0, absCmp)
	am.Update(-10)
	am.Update(5)
	am.Update(-3)
	s.Assert().Equal(-10, am.Get())
}

func (s *atomicMaxTestSuite) TestConcurrentUpdates() {
	am := NewAtomicMax(0, cmp.Compare[int])

	var wg sync.WaitGroup
	for i := range 1000 {
		wg.Go(func() {
			am.Update(i)
		})
	}
	wg.Wait()

	s.Assert().Equal(999, am.Get())
}

func (s *atomicMaxTestSuite) TestConcurrentUpdatesAndReads() {
	am := NewAtomicMax(0, cmp.Compare[int])

	updatesDone := make(chan struct{})
	getsDone := make(chan struct{})
	go func() {
		defer close(getsDone)
		for {
			select {
			case <-updatesDone:
				return
			default:
				v := am.Get()
				s.Assert().GreaterOrEqual(v, 0)
				s.Assert().Less(v, 1000)
				runtime.Gosched()
			}
		}
	}()

	var wg sync.WaitGroup
	for i := range 1000 {
		wg.Go(func() {
			am.Update(i)
		})
	}
	wg.Wait()

	close(updatesDone)
	s.Assert().Equal(999, am.Get())

	<-getsDone
}
