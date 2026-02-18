package bsontools

import (
	"slices"
	"testing"

	"github.com/samber/lo"
	"github.com/samber/lo/mutable"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestCompareBinaries(t *testing.T) {
	// NB: BSON sorts short binary strings before long ones, regardless of
	// content. This differs from standard binary sort order.
	expectedOrder := []bson.Binary{
		// First is a short buffer with a low subtype.
		{
			Subtype: 1,
			Data:    []byte("o"),
		},
		// Then comes a short buffer with a high subtype.
		{
			Subtype: 128,
			Data:    []byte("e"),
		},
		// Now a not-as-short buffer with a very-low subtype.
		{
			Data: []byte{0x2c, 0x01, 0xd4},
		},
		// Now the longest buffer (still very-low subtype).
		{
			Data: []byte("<heyheyheyhey"),
		},
	}

	toSort := expectedOrder
	mutable.Shuffle(toSort)

	slices.SortFunc(
		toSort,
		func(a, b bson.Binary) int {
			return lo.Must(
				CompareBinaries(
					ToRawValue(a),
					ToRawValue(b),
				),
			)
		},
	)

	assert.Equal(t, expectedOrder, toSort, "comparison orders as we expect")
}

func TestCompareRawValues(t *testing.T) {
	for _, cur := range []struct {
		a, b   any
		expect int
	}{
		{int64(0), int64(0), 0},
		{int64(1), int64(0), 1},
		{int64(0), int64(1), -1},
		{"", "", 0},
		{"", "\x00", -1},
		{"b", "aaa", 1},
		{
			bson.Binary{
				Data: []byte("abc"),
			},
			bson.Binary{
				Data: []byte("abc"),
			},
			0,
		},
		{
			bson.Binary{
				Data: []byte("cba"),
			},
			bson.Binary{
				Data: []byte("abc"),
			},
			1,
		},
		{
			bson.Binary{
				Data: []byte("abc"),
			},
			bson.Binary{
				Data: []byte("cba"),
			},
			-1,
		},
	} {
		aType, aBuf, err := bson.MarshalValue(cur.a)
		require.NoError(t, err)

		bType, bBuf, err := bson.MarshalValue(cur.b)
		require.NoError(t, err)

		got, err := CompareRawValues(
			bson.RawValue{Type: aType, Value: aBuf},
			bson.RawValue{Type: bType, Value: bBuf},
		)
		require.NoError(t, err, "must compare %v (BSON %s) & %v (BSON %s)", cur.a, aType, cur.b, bType)

		assert.Equal(t, cur.expect, got, "%v cmp %v", cur.a, cur.b)
	}
}
