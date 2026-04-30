package bsontools

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

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
	for {
		opt, err := iter.Next()
		require.NoError(t, err)
		if opt.IsNone() {
			break
		}
		got++
	}
	require.Equal(t, want, got, "iterator should visit every field")

	avg := testing.AllocsPerRun(100, func() {
		iter, _ := NewRawIterator(raw)
		for {
			opt, _ := iter.Next()
			if opt.IsNone() {
				return
			}
			// Force the element out of the Option so the body of Next is
			// not dead-code-eliminated. MustGet is itself alloc-free per
			// the option package's tests.
			_ = opt.MustGet()
		}
	})

	assert.Zero(t, avg, "happy-path iteration should not heap-allocate")
}
