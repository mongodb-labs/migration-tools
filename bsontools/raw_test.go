package bsontools

import (
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/x/bsonx/bsoncore"
)

func TestRawLookup(t *testing.T) {
	doc := bson.D{
		{"foo", "bar"},
	}
	raw, err := bson.Marshal(doc)
	require.NoError(t, err)

	bar, err := RawLookup[string](raw, "foo")
	require.NoError(t, err)
	assert.Equal(t, "bar", bar)

	_, err = RawLookup[string](raw, "oops")
	assert.ErrorIs(t, err, bsoncore.ErrElementNotFound)

	_, err = RawLookup[string](raw, "foo", "bar")
	assert.ErrorAs(t, err, &bsoncore.InvalidDepthTraversalError{})

	_, err = RawLookup[int64](raw, "foo")
	assert.Error(t, err)
	assert.ErrorContains(t, err, "int64")
	assert.ErrorContains(t, err, "string")
}

// TestCountRawElementsNoAllocs verifies that CountRawElements does not
// heap-allocate when iterating a multi-field document. Regression here would
// indicate that one of the iterator's internal allocations leaked back in.
func TestCountRawElementsNoAllocs(t *testing.T) {
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

	// Sanity-check that we're actually counting all fields, so the no-alloc
	// loop below is exercising the real happy path.
	count, err := CountRawElements(raw)
	require.NoError(t, err)
	require.Equal(t, len(doc), count)

	avg := testing.AllocsPerRun(100, func() {
		_, err = CountRawElements(raw)
		require.NoError(t, err)
	})

	assert.Zero(t, avg, "CountRawElements should not heap-allocate")
}

// TestCountRawElementsEmptyBuffer verifies that CountRawElements treats an
// empty buffer as zero fields (with no error), even though NewRawIterator now
// rejects empty input. CountRawElements short-circuits this case.
func TestCountRawElementsEmptyBuffer(t *testing.T) {
	count, err := CountRawElements(bson.Raw{})
	require.NoError(t, err)
	assert.Zero(t, count)

	count, err = CountRawElements(bson.Raw(nil))
	require.NoError(t, err)
	assert.Zero(t, count)
}

func TestRawElements_Empty(t *testing.T) {
	var doc bson.Raw

	for _, err := range RawElements(doc) {
		require.NoError(t, err)
		require.Fail(t, "should have no elements")
	}

	doc = bson.Raw{}

	for _, err := range RawElements(doc) {
		require.NoError(t, err)
		require.Fail(t, "should have no elements")
	}
}

func TestRawElements_ShortHeader(t *testing.T) {
	doc := bson.Raw{0, 1, 2}

	for _, err := range RawElements(doc) {
		require.Error(t, err)
		assert.ErrorAs(t, err, &bsoncore.InsufficientBytesError{})
	}
}

func TestRawElements(t *testing.T) {
	srcD := bson.D{
		{"foo", "xxx"},
		{"bar", "baz"},
	}

	mydoc, err := bson.Marshal(srcD)
	require.NoError(t, err)

	count, err := CountRawElements(mydoc)
	require.NoError(t, err)

	assert.Equal(t, len(srcD), count)

	received := bson.D{}

	for el, err := range RawElements(mydoc) {
		require.NoError(t, err, "should iterate")

		received = append(received, bson.E{
			lo.Must(el.KeyErr()),
			lo.Must(el.ValueErr()).StringValue(),
		})
	}

	assert.Equal(t, srcD, received, "should iterate all fields")

	// Now make the document invalid
	mydoc = mydoc[:len(mydoc)-3]

	received = received[:0]

	var iterErr error
	for _, err := range RawElements(mydoc) {
		if err != nil {
			iterErr = err
			break
		}
	}

	assert.Error(t, iterErr, "should fail somewhere in the iteration")

	assert.Panics(
		t,
		func() {
			var gotErr error
			for _, err := range RawElements(mydoc) {
				if err != nil {
					gotErr = err
				}
			}

			assert.Error(t, gotErr, "should get an error")
		},
		"should panic if we fail but didn’t stop iterating",
	)
}
