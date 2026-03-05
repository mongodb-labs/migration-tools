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
	bson.Regex{Pattern: "ab?c", Options: "o"},
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

		inRaw, found, err := RemoveFromRaw(inRaw, "foo")
		require.NoError(t, err)
		assert.False(t, found, "should not find element")

		assert.Equal(t, inRawCopy, inRaw, "should not change doc")
	}
}

func TestRemoveFromRaw_Oplog(t *testing.T) {
	ejson := `{"ts": {"$timestamp":{"t":1769128758,"i":2}},"t": {"$numberLong":"1"},"v": {"$numberInt":"2"},"op": "i","ns": "txnDB.stuff","o": {"_id": "outside_txn"},"o2": {"_id": "outside_txn"},"ui": {"$binary":{"base64":"1/FBdMdnRsuVGiCsEuPHIw==","subType":"04"}},"lsid": {"id": {"$binary":{"base64":"Bhv0WYEgSyWyTHgzVLI4bg==","subType":"04"}},"uid": {"$binary":{"base64":"47DEQpj8HBSa+/TImW+5JCeuQeRkm5NMpJWZG3hSuFU=","subType":"00"}}},"txnNumber": {"$numberLong":"1"},"prevOpTime": {"ts": {"$timestamp":{"t":0,"i":0}},"t": {"$numberLong":"-1"}}}`

	var raw bson.Raw
	require.NoError(t, bson.UnmarshalExtJSON([]byte(ejson), false, &raw))

	raw, found, err := RemoveFromRaw(raw, "ui")
	require.NoError(t, err)
	assert.True(t, found)

	assert.Zero(t, raw.Lookup("ui"))
}

func TestReplaceInRaw_Missing(t *testing.T) {
	for _, val := range referenceValues {
		inDoc := bson.D{
			{"bar", val},
			{"baz", "234"},
		}

		inRaw := bson.Raw(lo.Must(bson.Marshal(inDoc)))

		inRawCopy := slices.Clone(inRaw)

		inRaw, found, err := ReplaceInRaw(
			inRaw,
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

		inRaw, found, err := RemoveFromRaw(inRaw, "bar")
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

		inRaw, found, err := RemoveFromRaw(inRaw, "bar")
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
	_, _, err := RemoveFromRaw(
		lo.Must(bson.Marshal(bson.D{
			{"foo", bson.D{{"bar", 234.345}}},
		})),
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

			inRaw, found, err := ReplaceInRaw(
				inRaw,
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

		inRaw, found, err := RemoveFromRaw(inRaw, "bar", "bbb", "1")
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

		inRaw, found, err := RemoveFromRaw(inRaw, "bar")
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

func TestReplaceInRaw_MissingKey(t *testing.T) {
	// SETUP: An embedded document followed by a sibling field.
	originalDoc := bson.D{
		{"parent", bson.D{
			{"existing_key", "value"},
		}},
		{"trailing_sibling", "do_not_lose_me"},
	}

	rawDoc, _ := bson.Marshal(originalDoc)
	origDoc := slices.Clone(rawDoc)
	newRawValue := ToRawValue("new\x00")

	newDoc, found, err := ReplaceInRaw(rawDoc, newRawValue, "parent", "missing_key")
	require.NoError(t, err, "ReplaceInRaw")
	require.False(t, found, "should not find missing key")

	assert.Equal(t, origDoc, newDoc, "doc must be unaltered")
}

func TestReplaceInRaw_EmbeddedDocumentBugs(t *testing.T) {
	// SETUP: Construct the document to trigger both bugs.
	// It MUST have an embedded document, followed by another field.
	originalDoc := bson.D{
		{"parent", bson.D{
			{"target", "old_value"},
		}},
		{"trailing_sibling", "do_not_lose_me"}, // This field is the victim of both bugs
	}

	rawDoc, err := bson.Marshal(originalDoc)
	require.NoError(t, err, "marshal original doc")

	// Prepare the replacement value ("new_value")
	bType, replacementBytes, err := bson.MarshalValue("new_value")
	require.NoError(t, err, "marshal replacement value")

	newRawValue := bson.RawValue{
		Type:  bType,
		Value: replacementBytes,
	}

	// ACT: Attempt to replace "parent.target"
	resultRaw, found, err := ReplaceInRaw(rawDoc, newRawValue, "parent", "target")

	// Check for overslicing / parsing crash
	require.NoError(t, err, "ReplaceInRaw should succeed")
	assert.True(t, found, "should find and replace the value")

	// Convert the raw bytes to bson.Raw to use traversal methods
	resultBSON := bson.Raw(resultRaw)
	require.NoError(t, resultBSON.Validate(), "result doc must be valid")

	// ASSERT THE REPLACEMENT WORKED
	targetRV, err := resultBSON.LookupErr("parent", "target")
	require.NoError(t, err, "must find 'parent.target'")
	assert.Equal(t, "new_value", targetRV.StringValue(), "parent.target new value")

	// Check for truncation bugs
	siblingRV, err := resultBSON.LookupErr("trailing_sibling")
	require.NoError(t, err, "'trailing_sibling' must remain")
	assert.Equal(t, "do_not_lose_me", siblingRV.StringValue(), "'trailing_sibling' must be unmodified")
}
