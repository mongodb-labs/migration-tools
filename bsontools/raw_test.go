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
