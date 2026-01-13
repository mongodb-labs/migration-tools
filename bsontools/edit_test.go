package bsontools

import (
	"encoding/binary"
	"math/rand/v2"
	"slices"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

var referenceValues = []any{
	bson.MinKey{},
	rand.Float64(),
	"abcdefg",
	bson.Binary{
		Data: binary.BigEndian.AppendUint64(nil, rand.Uint64()),
	},
	bson.Undefined{},
	bson.NewObjectID(),
	false,
	true,
	bson.NewDateTimeFromTime(time.Now()),
	nil,
	bson.Regex{"ab?c", "o"},
	bson.DBPointer{DB: "mydb", Pointer: bson.NewObjectID()},
	bson.JavaScript("use strict;"),
	bson.Symbol("nonono"),
	bson.CodeWithScope{
		Code: bson.JavaScript("'hello'"),
		Scope: bson.D{
			{"what", "hey"},
		},
	},
	rand.Int32(),
	bson.Timestamp{rand.Uint32(), rand.Uint32()},
	rand.Int64(),
	lo.Must(bson.ParseDecimal128("12345")),
	bson.MaxKey{},
}

func TestRemoveFromRaw_Missing(t *testing.T) {
	for _, val := range referenceValues {
		inDoc := bson.D{
			{"bar", val},
			{"baz", "234"},
		}

		inRaw := bson.Raw(lo.Must(bson.Marshal(inDoc)))

		inRawCopy := slices.Clone(inRaw)

		found, err := RemoveFromRaw(&inRaw, "foo")
		require.NoError(t, err)
		assert.False(t, found, "should not find element")

		assert.Equal(t, inRawCopy, inRaw, "should not change doc")
	}
}

func TestReplaceInRaw_Missing(t *testing.T) {
	for _, val := range referenceValues {
		inDoc := bson.D{
			{"bar", val},
			{"baz", "234"},
		}

		inRaw := bson.Raw(lo.Must(bson.Marshal(inDoc)))

		inRawCopy := slices.Clone(inRaw)

		found, err := ReplaceInRaw(
			&inRaw,
			bson.RawValue{Type: bson.TypeNull},
			"foo",
		)
		require.NoError(t, err)
		assert.False(t, found, "should not find element")

		assert.Equal(t, inRawCopy, inRaw, "should not change doc")
	}
}

func TestRemoveFromRaw_Shallow_RemoveFromStart(t *testing.T) {
	for _, val := range referenceValues {
		inDoc := bson.D{
			{"bar", val},
			{"baz", "234"},
		}

		inRaw := bson.Raw(lo.Must(bson.Marshal(inDoc)))

		bsonType := inRaw.Lookup("bar").Type

		found, err := RemoveFromRaw(&inRaw, "bar")
		require.NoError(t, err)
		assert.True(t, found, "should remove")

		var outDoc bson.D
		require.NoError(t, bson.Unmarshal(inRaw, &outDoc))

		assert.Equal(
			t,
			bson.D{
				{"baz", "234"},
			},
			outDoc,
			"remove %s",
			bsonType,
		)
	}
}

func TestRemoveFromRaw_Shallow_RemoveFromMiddle(t *testing.T) {
	for _, val := range referenceValues {
		inDoc := bson.D{
			{"foo", "123"},
			{"bar", val},
			{"baz", "345"},
		}

		inRaw := bson.Raw(lo.Must(bson.Marshal(inDoc)))

		bsonType := inRaw.Lookup("bar").Type

		found, err := RemoveFromRaw(&inRaw, "bar")
		require.NoError(t, err)
		assert.True(t, found, "should remove")

		var outDoc bson.D
		require.NoError(t, bson.Unmarshal(inRaw, &outDoc))

		assert.Equal(
			t,
			bson.D{
				{"foo", "123"},
				{"baz", "345"},
			},
			outDoc,
			"remove %s",
			bsonType,
		)
	}
}

func TestRemoveFromRaw_Deep_PointerTooDeep(t *testing.T) {
	_, err := RemoveFromRaw(
		lo.ToPtr(bson.Raw(lo.Must(bson.Marshal(bson.D{
			{"foo", bson.D{{"bar", 234.345}}},
		})))),
		"foo",
		"bar",
		"baz",
	)

	var deepErr PointerTooDeepError
	require.ErrorAs(t, err, &deepErr)
	assert.Equal(
		t,
		[]string{"foo", "bar", "baz"},
		deepErr.givenPointer,
	)
	assert.Equal(
		t,
		[]string{"foo", "bar"},
		deepErr.elementPointer,
	)
	assert.Equal(
		t,
		bson.TypeDouble,
		deepErr.elementType,
	)
}

func TestReplaceInRaw_Deep(t *testing.T) {
	for _, val := range referenceValues {
		for _, replacementVal := range referenceValues {
			inDoc := bson.D{
				{"foo", "123"},
				{"bar", bson.D{
					{"aaa", 123.234},
					{"bbb", []any{nil, val, true}},
					{"ccc", false},
				}},
				{"baz", "345"},
			}

			inRaw := bson.Raw(lo.Must(bson.Marshal(inDoc)))

			bsonType := inRaw.Lookup("bar", "bbb").Type

			var replacement bson.RawValue
			if replacementVal == nil {
				replacement.Type = bson.TypeNull
			} else {
				replacement.Type, replacement.Value = lo.Must2(bson.MarshalValue(replacementVal))
			}

			found, err := ReplaceInRaw(
				&inRaw,
				replacement,
				"bar", "bbb", "1",
			)
			require.NoError(t, err)
			assert.True(t, found, "should find")

			var outDoc bson.D
			require.NoError(t, bson.Unmarshal(inRaw, &outDoc))

			assert.Equal(
				t,
				bson.D{
					{"foo", "123"},
					{"bar", bson.D{
						{"aaa", 123.234},
						{"bbb", bson.A{nil, replacementVal, true}},
						{"ccc", false},
					}},
					{"baz", "345"},
				},
				outDoc,
				"replace %s with %s",
				bsonType,
				replacement.Type,
			)
		}
	}
}

func TestRemoveFromRaw_Deep(t *testing.T) {
	for _, val := range referenceValues {
		inDoc := bson.D{
			{"foo", "123"},
			{"bar", bson.D{
				{"aaa", 123.234},
				{"bbb", []any{nil, val, true}},
				{"ccc", false},
			}},
			{"baz", "345"},
		}

		inRaw := bson.Raw(lo.Must(bson.Marshal(inDoc)))

		bsonType := inRaw.Lookup("bar", "bbb").Type

		found, err := RemoveFromRaw(&inRaw, "bar", "bbb", "1")
		require.NoError(t, err)
		assert.True(t, found, "should remove")

		var outDoc bson.D
		require.NoError(t, bson.Unmarshal(inRaw, &outDoc))

		assert.Equal(
			t,
			bson.D{
				{"foo", "123"},
				{"bar", bson.D{
					{"aaa", 123.234},
					{"bbb", bson.A{nil, true}},
					{"ccc", false},
				}},
				{"baz", "345"},
			},
			outDoc,
			"remove %s",
			bsonType,
		)
	}
}

func TestRemoveFromRaw_Shallow_RemoveFromEnd(t *testing.T) {
	for _, val := range referenceValues {
		inDoc := bson.D{
			{"foo", "123"},
			{"bar", val},
		}

		inRaw := bson.Raw(lo.Must(bson.Marshal(inDoc)))

		bsonType := inRaw.Lookup("bar").Type

		found, err := RemoveFromRaw(&inRaw, "bar")
		require.NoError(t, err)
		assert.True(t, found, "should remove")

		var outDoc bson.D
		require.NoError(t, bson.Unmarshal(inRaw, &outDoc))

		assert.Equal(
			t,
			bson.D{
				{"foo", "123"},
			},
			outDoc,
			"remove %s",
			bsonType,
		)
	}
}
