package bsontools

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

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
