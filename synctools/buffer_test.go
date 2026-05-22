package synctools

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type bufferTestSuite struct {
	suite.Suite
}

func TestBufferTestSuite(t *testing.T) {
	suite.Run(t, &bufferTestSuite{})
}

func (s *bufferTestSuite) write(b *Buffer, data string) {
	_, err := b.Write([]byte(data))
	s.Require().NoError(err)
}

func (s *bufferTestSuite) TestWriteAndBytes() {
	var b Buffer
	n, err := b.Write([]byte("hello"))
	s.Require().NoError(err)
	s.Assert().Equal(5, n)
	s.Assert().Equal([]byte("hello"), b.Bytes())
}

func (s *bufferTestSuite) TestMultipleWritesAccumulate() {
	var b Buffer
	s.write(&b, "foo")
	s.write(&b, "bar")
	s.Assert().Equal([]byte("foobar"), b.Bytes())
}

func (s *bufferTestSuite) TestBytesIsClone() {
	var b Buffer
	s.write(&b, "hello")

	got := b.Bytes()
	got[0] = 'X'

	// Buffer should be unaffected.
	s.Assert().Equal([]byte("hello"), b.Bytes())
}

func (s *bufferTestSuite) TestReset() {
	var b Buffer
	s.write(&b, "hello")
	b.Reset()
	s.Assert().Empty(b.Bytes())

	// Write after Reset starts fresh.
	s.write(&b, "world")
	s.Assert().Equal([]byte("world"), b.Bytes())
}

// TestConcurrentWrites verifies that concurrent Write calls are race-free.
// Run with go test -race.
func (s *bufferTestSuite) TestConcurrentWrites() {
	var b Buffer
	const goroutines = 50

	var wg sync.WaitGroup
	for range goroutines {
		wg.Go(func() {
			_, err := b.Write([]byte("x"))
			assert.NoError(s.T(), err)
		})
	}
	wg.Wait()

	s.Assert().Len(b.Bytes(), goroutines)
}

// TestConcurrentBytesReads verifies that concurrent Bytes calls are race-free.
func (s *bufferTestSuite) TestConcurrentBytesReads() {
	var b Buffer
	s.write(&b, "hello")

	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() {
			got := b.Bytes()
			assert.Equal(s.T(), []byte("hello"), got)
		})
	}
	wg.Wait()
}
