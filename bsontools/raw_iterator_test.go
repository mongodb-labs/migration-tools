package bsontools

import (
	"bytes"
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

// TestNewRawIteratorBelowMinimumLength verifies that a non-empty buffer with
// a declared length below the BSON minimum (5 bytes) is rejected, even when
// the declared length matches the actual buffer length.
func TestNewRawIteratorBelowMinimumLength(t *testing.T) {
	// 4-byte buffer whose length header self-consistently claims 4 bytes:
	// header is correct length, length matches buffer, but no room for
	// terminator. Without the explicit minimum-length check this would
	// silently appear to be an "empty" document.
	doc := []byte{0x04, 0x00, 0x00, 0x00}

	_, err := NewRawIterator(doc)
	require.Error(t, err)
	assert.ErrorContains(t, err, "below BSON minimum")
}

// TestNewRawIteratorMissingTerminator verifies that a BSON document without
// the required 0x00 trailing terminator byte is rejected. Without this check
// the iterator would treat a malformed doc as a clean end-of-document.
func TestNewRawIteratorMissingTerminator(t *testing.T) {
	// 5-byte buffer claiming to be 5 bytes long, but final byte is 0xFF
	// (should be 0x00).
	doc := []byte{0x05, 0x00, 0x00, 0x00, 0xFF}

	_, err := NewRawIterator(doc)
	require.Error(t, err)
	assert.ErrorContains(t, err, "trailing NUL terminator")
	assert.ErrorContains(t, err, "0xff")
}

// TestRawIteratorMalformedElement verifies that a doc whose outer structure
// is valid (header, length, terminator) but whose element body is malformed
// surfaces the error via Err, latches the error so Next stays nil, and does
// not advance Index past the failed field.
func TestRawIteratorMalformedElement(t *testing.T) {
	// {"a": "ok", "b": "oops"} — corrupt the second value's string-length
	// prefix to claim a wildly large size, while keeping the outer document
	// length and terminator intact so NewRawIterator accepts the buffer.
	doc := bson.D{{"a", "ok"}, {"b", "oops"}}
	raw, err := bson.Marshal(doc)
	require.NoError(t, err)

	tampered := append([]byte{}, raw...)
	// Locate the 4-byte string-length prefix of value "oops". The simplest
	// way is to scan for the unique substring "oops" and step back 4 bytes.
	idx := bytes.Index(tampered, []byte("oops")) - 4
	require.GreaterOrEqual(t, idx, 0, "should locate string body in marshaled doc")
	// Claim the string is much longer than the buffer's remaining bytes.
	// Use a small-but-larger-than-remaining value so bsoncore's ReadElement
	// returns !ok cleanly (as opposed to panicking on extreme values).
	binary.LittleEndian.PutUint32(tampered[idx:idx+4], 100)

	iter, err := NewRawIterator(tampered)
	require.NoError(t, err, "outer doc should still pass NewRawIterator validation")

	// First field parses fine.
	first := iter.Next()
	require.NotNil(t, first)
	assert.Equal(t, "a", first.Key())
	assert.Equal(t, 1, iter.Index(), "Index should advance after successful parse")

	// Second field is malformed.
	second := iter.Next()
	assert.Nil(t, second, "malformed element should not be returned")
	require.Error(t, iter.Err())
	assert.Equal(t, 1, iter.Index(), "Index should not advance past failed field")

	// Latched-error semantics: subsequent calls keep returning nil with the
	// same error.
	firstErr := iter.Err()
	assert.Nil(t, iter.Next())
	assert.Equal(t, firstErr, iter.Err())
	assert.Nil(t, iter.Next())
	assert.Equal(t, firstErr, iter.Err())
}

// TestRawIteratorElementConsumesTerminator verifies that an element whose
// declared length would extend into (and consume) the document's trailing
// 0x00 terminator surfaces as an error rather than being silently accepted.
//
// This regresses a bug where the iterator stored doc[4:] as remaining bytes
// (including the terminator) and exited cleanly when remaining was small,
// allowing malformed elements that swallowed the terminator to slip through.
func TestRawIteratorElementConsumesTerminator(t *testing.T) {
	// {"a": "ab"} marshals to 15 bytes:
	// [len=15][type=02][key=a\0][strlen=3][a b \0][doc-term=0]
	// Bump the inner string-length to 4 so the element claims one more
	// byte of value than is available before the terminator. The element
	// would then consume the doc terminator, leaving no terminator behind.
	doc, err := bson.Marshal(bson.D{{"a", "ab"}})
	require.NoError(t, err)

	// Locate the 4-byte string-length prefix immediately preceding "ab".
	idx := -1
	for i := 0; i+2 < len(doc); i++ {
		if string(doc[i:i+2]) == "ab" {
			idx = i - 4
			break
		}
	}
	require.GreaterOrEqual(t, idx, 0)
	binary.LittleEndian.PutUint32(doc[idx:idx+4], 4)

	iter, err := NewRawIterator(doc)
	require.NoError(t, err, "outer doc should still pass NewRawIterator")

	// Iterate to exhaustion. The malformed element must surface as an error.
	for iter.Next() != nil {
	}
	require.Error(t, iter.Err(), "malformed last element must not be silently accepted")
}

// TestRawIteratorIndex verifies that Index reports the 0-based index of the
// next field to be returned by Next.
func TestRawIteratorIndex(t *testing.T) {
	doc := bson.D{{"a", 1}, {"b", 2}, {"c", 3}}
	raw, err := bson.Marshal(doc)
	require.NoError(t, err)

	iter, err := NewRawIterator(raw)
	require.NoError(t, err)
	assert.Equal(t, 0, iter.Index(), "Index should be 0 before iteration")

	require.NotNil(t, iter.Next())
	assert.Equal(t, 1, iter.Index())

	require.NotNil(t, iter.Next())
	assert.Equal(t, 2, iter.Index())

	require.NotNil(t, iter.Next())
	assert.Equal(t, 3, iter.Index())

	assert.Nil(t, iter.Next(), "no more fields")
	assert.Equal(t, 3, iter.Index(), "Index does not advance past end-of-document")
	assert.NoError(t, iter.Err())
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
