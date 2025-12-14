package bsontools

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestWrongType(t *testing.T) {
	type myString string

	rv := mustConvertToRawValue(t, "abc")
	got, err := RawValueTo[int64](rv)
	require.Error(t, err)
	assert.ErrorContains(t, err, fmt.Sprintf("%T", got))
}

func TestBool(t *testing.T) {
	vals := []bool{false, true}

	for _, cur := range vals {
		viaMarshal := mustConvertToRawValue(t, cur)

		assert.Equal(t, cur, lo.Must(RawValueTo[bool](viaMarshal)), "round-trip")
	}
}

func TestInt32(t *testing.T) {
	ints := []int32{
		0,
		-1,
		math.MaxInt32 - 1,
		math.MaxInt32,
		math.MinInt32,
		math.MinInt32 + 1,
	}

	for _, cur := range ints {
		viaMarshal := mustConvertToRawValue(t, cur)

		assert.Equal(t, cur, lo.Must(RawValueTo[int32](viaMarshal)), "round-trip")
	}
}

func TestInt64(t *testing.T) {
	ints := []int64{
		0,
		-1,
		math.MaxInt32 - 1,
		math.MaxInt32,
		math.MaxInt32 + 1,
		math.MaxInt64,
		math.MinInt32 - 1,
		math.MinInt32,
		math.MinInt32 + 1,
		math.MinInt64,
	}

	for _, cur := range ints {
		viaMarshal := mustConvertToRawValue(t, cur)

		assert.Equal(t, cur, lo.Must(RawValueTo[int64](viaMarshal)), "round-trip")
	}
}

func TestFloat64(t *testing.T) {
	ints := []float64{
		1.25,
		-99.825,
		0,
		-1,
		math.MaxInt32 - 1,
		math.MaxInt32,
		math.MinInt32,
		math.MinInt32 + 1,
	}

	for _, cur := range ints {
		viaMarshal := mustConvertToRawValue(t, cur)

		assert.Equal(t, cur, lo.Must(RawValueTo[float64](viaMarshal)), "round-trip")
	}
}

func TestString(t *testing.T) {
	vals := []string{
		"",
		"0",
		"abc",
		"ábç",
	}

	for _, cur := range vals {
		viaMarshal := mustConvertToRawValue(t, cur)

		assert.Equal(t, cur, lo.Must(RawValueTo[string](viaMarshal)))
	}
}

func TestRaw(t *testing.T) {
	vals := []bson.Raw{
		lo.Must(bson.Marshal(bson.D{})),
		lo.Must(bson.Marshal(bson.D{{"", nil}})),
		lo.Must(bson.Marshal(bson.D{{"a", 1.2}})),
	}

	for _, cur := range vals {
		viaMarshal := mustConvertToRawValue(t, cur)

		assert.Equal(t, cur, lo.Must(RawValueTo[bson.Raw](viaMarshal)))
	}
}

func TestRawArray(t *testing.T) {
	vals := lo.Map(
		[]bson.RawArray{
			lo.Must(bson.Marshal(bson.D{})),
			lo.Must(bson.Marshal(bson.D{{"0", nil}})),
			lo.Must(bson.Marshal(bson.D{{"0", 1.2}, {"1", "abc"}})),
		},
		func(ra bson.RawArray, _ int) bson.RawValue {
			return bson.RawValue{
				Type:  bson.TypeArray,
				Value: []byte(ra),
			}
		},
	)

	for _, cur := range vals {
		ra, err := RawValueTo[bson.RawArray](cur)
		require.NoError(t, err)

		assert.Equal(t, cur.Value, []byte(ra), "expect same bytes")
	}
}

func TestTimestamp(t *testing.T) {
	vals := []bson.Timestamp{
		{0, 0},
		{1, 1},
		{math.MaxUint32, math.MaxUint32},
	}

	for _, cur := range vals {
		viaMarshal := mustConvertToRawValue(t, cur)

		assert.Equal(t, cur, lo.Must(RawValueTo[bson.Timestamp](viaMarshal)))
	}
}

func TestObjectID(t *testing.T) {
	vals := []bson.ObjectID{
		bson.NewObjectID(),
		{},
	}

	for _, cur := range vals {
		viaMarshal := mustConvertToRawValue(t, cur)

		assert.Equal(t, cur, lo.Must(RawValueTo[bson.ObjectID](viaMarshal)))
	}
}

func TestTime(t *testing.T) {
	vals := []time.Time{
		time.UnixMilli(time.Now().UnixMilli()),
		time.UnixMilli(0),
	}

	for _, cur := range vals {
		viaMarshal := mustConvertToRawValue(t, cur)

		assert.Equal(t, cur, lo.Must(RawValueTo[time.Time](viaMarshal)))
	}
}

func TestDateTime(t *testing.T) {
	vals := []time.Time{
		time.UnixMilli(time.Now().UnixMilli()),
		time.UnixMilli(0),
	}

	for _, cur := range vals {
		curDT := bson.DateTime(cur.UnixMilli())

		viaMarshal := mustConvertToRawValue(t, curDT)

		assert.Equal(t, curDT, lo.Must(RawValueTo[bson.DateTime](viaMarshal)))
	}
}

func TestDecimal128(t *testing.T) {
	vals := []bson.Decimal128{
		{},
		bson.NewDecimal128(0, 0),
		bson.NewDecimal128(1, 1),
	}

	for _, cur := range vals {
		viaMarshal := mustConvertToRawValue(t, cur)

		assert.Equal(t, cur, lo.Must(RawValueTo[bson.Decimal128](viaMarshal)))
	}
}

func TestBinary(t *testing.T) {
	vals := []bson.Binary{
		{0, []byte{}},
		{0x4, []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}},
	}

	for _, cur := range vals {
		viaMarshal := mustConvertToRawValue(t, cur)

		assert.Equal(t, cur, lo.Must(RawValueTo[bson.Binary](viaMarshal)))
	}
}

func TestRegex(t *testing.T) {
	vals := []bson.Regex{
		{},
		{Pattern: "abc", Options: "i"},
	}

	for _, cur := range vals {
		viaMarshal := mustConvertToRawValue(t, cur)

		assert.Equal(t, cur, lo.Must(RawValueTo[bson.Regex](viaMarshal)))
	}
}

func mustConvertToRawValue(t *testing.T, val any) bson.RawValue {
	bsonType, buf, err := bson.MarshalValue(val)
	require.NoError(t, err, "must marshal Go %T to BSON", val)

	return bson.RawValue{
		Type:  bsonType,
		Value: buf,
	}
}
