package bsontools

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/x/bsonx/bsoncore"
)

// TestNewRawIteratorEmpty verifies that an empty buffer is treated as a
// valid empty BSON document: no error, no elements yielded. This matches the
// 5-byte all-NUL canonical empty document.
func TestNewRawIteratorEmpty(t *testing.T) {
	for _, tc := range []struct {
		name string
		doc  bson.Raw
	}{
		{"nil slice", bson.Raw(nil)},
		{"zero-length slice", bson.Raw{}},
		{"5-byte all NUL", bson.Raw{0x05, 0, 0, 0, 0}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			iter, err := NewRawIterator(tc.doc)
			require.NoError(t, err)
			assert.Nil(t, iter.Next(), "no elements expected")
			assert.NoError(t, iter.Err())
		})
	}
}

// TestNewRawIteratorShortBuffer verifies that a buffer too short to hold the
// 4-byte length header is rejected with a wrapped InsufficientBytesError.
func TestNewRawIteratorShortBuffer(t *testing.T) {
	_, err := NewRawIterator(bson.Raw{0, 1, 2})
	require.Error(t, err)
	assert.ErrorAs(t, err, &bsoncore.InsufficientBytesError{})
	assert.ErrorContains(t, err, "3 bytes long")
}

// TestNewRawIteratorLengthMismatch verifies that a declared document length
// not matching the actual buffer length is rejected.
func TestNewRawIteratorLengthMismatch(t *testing.T) {
	doc := bson.D{{"foo", "bar"}}
	raw, err := bson.Marshal(doc)
	require.NoError(t, err)

	t.Run("buffer truncated", func(t *testing.T) {
		_, err := NewRawIterator(raw[:len(raw)-1])
		require.Error(t, err)
		assert.ErrorContains(t, err, "declared document length")
		assert.ErrorContains(t, err, "actual buffer length")
	})

	t.Run("buffer extended", func(t *testing.T) {
		extended := append([]byte{}, raw...)
		extended = append(extended, 0x00)
		_, err := NewRawIterator(extended)
		require.Error(t, err)
		assert.ErrorContains(t, err, "declared document length")
		assert.ErrorContains(t, err, "actual buffer length")
	})

	t.Run("declared length lies", func(t *testing.T) {
		// Take a valid doc and overwrite the length header with a wrong value.
		tampered := append([]byte{}, raw...)
		binary.LittleEndian.PutUint32(tampered, uint32(len(raw)+100))
		_, err := NewRawIterator(tampered)
		require.Error(t, err)
		assert.ErrorContains(t, err, "declared document length")
	})
}

// TestRawIteratorNoAllocs verifies that the happy-path iteration of a
// RawIterator over a multi-field document does not heap-allocate. The
// document holds a representative mix of types (string, int32, int64, bool,
// double, embedded doc, array) so that a regression in any of the type-
// specific Validate paths would surface here.
func TestRawIteratorNoAllocs(t *testing.T) {
	doc := bson.D{
		{"s", "hello"},
		{"i32", int32(42)},
		{"i64", int64(1 << 40)},
		{"b", true},
		{"f", 3.14},
		{"sub", bson.D{{"x", "y"}}},
		{"arr", bson.A{1, 2, 3}},
	}
	raw, err := bson.Marshal(doc)
	require.NoError(t, err)

	// Sanity check: the iterator returns the expected number of fields, so
	// the no-alloc benchmark below is iterating over the real happy path
	// (not bailing out on the first call due to a malformed input).
	want := len(doc)
	iter, err := NewRawIterator(raw)
	require.NoError(t, err)
	got := 0
	for el := iter.Next(); el != nil; el = iter.Next() {
		got++
	}
	require.NoError(t, iter.Err())
	require.Equal(t, want, got, "iterator should visit every field")

	avg := testing.AllocsPerRun(100, func() {
		iter, err := NewRawIterator(raw)
		require.NoError(t, err)

		for el := iter.Next(); el != nil; el = iter.Next() {
			// Reference el so the loop body isn't dead-code-eliminated.
			_ = el
		}
	})

	assert.Zero(t, avg, "happy-path iteration should not heap-allocate")
}
