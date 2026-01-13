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

// The type tests below are in numeric BSON type order:

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
		fromUs := ToRawValue(cur)
		viaMarshal := mustConvertToRawValue(t, cur)

		assert.Equal(t, viaMarshal, fromUs)
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

func TestUndefined(t *testing.T) {
	fromUs := ToRawValue(bson.Undefined{})
	viaMarshal := mustConvertToRawValue(t, bson.Undefined{})

	assert.Equal(t, bson.Undefined{}, lo.Must(RawValueTo[bson.Undefined](fromUs)))
	assert.Equal(t, bson.Undefined{}, lo.Must(RawValueTo[bson.Undefined](viaMarshal)))
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

func TestBool(t *testing.T) {
	vals := []bool{false, true}

	for _, cur := range vals {
		viaMarshal := mustConvertToRawValue(t, cur)

		assert.Equal(t, cur, lo.Must(RawValueTo[bool](viaMarshal)), "round-trip")
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

func TestNull(t *testing.T) {
	fromUs := ToRawValue(bson.Null{})
	viaMarshal := mustConvertToRawValue(t, bson.Null{})

	assert.Equal(t, bson.Null{}, lo.Must(RawValueTo[bson.Null](fromUs)))
	assert.Equal(t, bson.Null{}, lo.Must(RawValueTo[bson.Null](viaMarshal)))
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

func TestDBPointer(t *testing.T) {
	for _, val := range []bson.DBPointer{
		{},
		{DB: "abc", Pointer: bson.NewObjectID()},
	} {
		fromUs := ToRawValue(val)
		viaMarshal := mustConvertToRawValue(t, val)

		assert.Equal(t, val, lo.Must(RawValueTo[bson.DBPointer](fromUs)))
		assert.Equal(t, val, lo.Must(RawValueTo[bson.DBPointer](viaMarshal)))
	}
}

func TestJavaScript(t *testing.T) {
	for _, val := range []bson.JavaScript{
		"",
		"abc",
	} {
		fromUs := ToRawValue(val)
		viaMarshal := mustConvertToRawValue(t, val)

		assert.Equal(t, val, lo.Must(RawValueTo[bson.JavaScript](fromUs)))
		assert.Equal(t, val, lo.Must(RawValueTo[bson.JavaScript](viaMarshal)))
	}
}

func TestSymbol(t *testing.T) {
	for _, val := range []bson.Symbol{
		"",
		"abc",
	} {
		fromUs := ToRawValue(val)
		viaMarshal := mustConvertToRawValue(t, val)

		assert.Equal(t, val, lo.Must(RawValueTo[bson.Symbol](fromUs)))
		assert.Equal(t, val, lo.Must(RawValueTo[bson.Symbol](viaMarshal)))
	}
}

func TypeCodeWithScope(t *testing.T) {
	for _, val := range []bson.CodeWithScope{
		{},
		{Code: "abc", Scope: bson.D{{"foo", "bar"}}},
	} {
		viaMarshal := mustConvertToRawValue(t, val)

		assert.Equal(t, val, lo.Must(RawValueTo[bson.CodeWithScope](viaMarshal)))
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

func TestMax(t *testing.T) {
	fromUs := ToRawValue(bson.MaxKey{})
	viaMarshal := mustConvertToRawValue(t, bson.MaxKey{})

	assert.Equal(t, bson.MaxKey{}, lo.Must(RawValueTo[bson.MaxKey](fromUs)))
	assert.Equal(t, bson.MaxKey{}, lo.Must(RawValueTo[bson.MaxKey](viaMarshal)))
}

func TestMin(t *testing.T) {
	fromUs := ToRawValue(bson.MinKey{})
	viaMarshal := mustConvertToRawValue(t, bson.MinKey{})

	assert.Equal(t, bson.MinKey{}, lo.Must(RawValueTo[bson.MinKey](fromUs)))
	assert.Equal(t, bson.MinKey{}, lo.Must(RawValueTo[bson.MinKey](viaMarshal)))
}

func TestInt_UnmarshalFraction(t *testing.T) {
	viaMarshal := mustConvertToRawValue(t, 1.25)

	_, err := RawValueTo[int](viaMarshal)
	assert.ErrorAs(t, err, &cannotCastError{})
}

func TestInt(t *testing.T) {
	ints := []int{
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

		assert.Equal(t, cur, lo.Must(RawValueTo[int](viaMarshal)), "round-trip")
	}

	coercible := []int64{
		0,
		-1,
		1,
		math.MaxInt32 - 1,
		math.MaxInt32,
		math.MinInt32,
		math.MinInt32 + 1,
	}

	for _, cur := range coercible {
		viaMarshal := mustConvertToRawValue(t, cur)

		assert.Equal(t, int(cur), lo.Must(RawValueTo[int](viaMarshal)), "round-trip")
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
