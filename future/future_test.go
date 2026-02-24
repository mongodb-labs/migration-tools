package future

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventual(t *testing.T) {
	eventual, setter := New[int]()

	assert.Panics(t, func() { eventual.Get() },
		"Get() should panic before the value is set",
	)

	select {
	case <-eventual.Ready():
		require.Fail(t, "should not be ready")
	case <-time.NewTimer(time.Millisecond).C:
	}

	setter(123)

	select {
	case <-eventual.Ready():
	case <-time.NewTimer(time.Millisecond).C:
		require.Fail(t, "should be ready")
	}

	assert.Equal(
		t,
		123,
		eventual.Get(),
		"Get() should return the value",
	)

	assert.Equal(
		t,
		123,
		eventual.Get(),
		"Get() should return the value a 2nd time",
	)
}

func TestEventualNil(t *testing.T) {
	eventual, setter := New[error]()

	select {
	case <-eventual.Ready():
		require.Fail(t, "should not be ready")
	case <-time.NewTimer(time.Millisecond).C:
	}

	setter(nil)

	select {
	case <-eventual.Ready():
	case <-time.NewTimer(time.Millisecond).C:
		require.Fail(t, "should be ready")
	}

	assert.Nil(t, eventual.Get())
}
