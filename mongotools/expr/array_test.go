package expr

import (
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestSlice(t *testing.T) {
	pos := any(1)

	// Without Position.
	assertExpr(t, Slice{Array: "$array", N: 2}, bson.D{
		{"$slice", bson.A{"$array", 2}},
	})

	// With Position.
	assertExpr(t, Slice{Array: "$array", Position: &pos, N: 2}, bson.D{
		{"$slice", bson.A{"$array", 1, 2}},
	})
}

func TestArrayElemAt(t *testing.T) {
	assertExpr(t, ArrayElemAt{Array: "$array", Index: -1}, bson.D{
		{"$arrayElemAt", bson.A{"$array", -1}},
	})
}

func TestSize(t *testing.T) {
	assertExpr(t, Size{"$array"}, bson.D{
		{"$size", "$array"},
	})
}

func TestSetDifference(t *testing.T) {
	assertExpr(t, SetDifference{"$current", "$previous"}, bson.D{
		{"$setDifference", bson.A{"$current", "$previous"}},
	})
}

func TestSetIntersection(t *testing.T) {
	assertExpr(t, SetIntersection{"$a", "$b", "$c"}, bson.D{
		{"$setIntersection", bson.A{"$a", "$b", "$c"}},
	})
}

func TestSetUnion(t *testing.T) {
	assertExpr(t, SetUnion{"$a", "$b"}, bson.D{
		{"$setUnion", bson.A{"$a", "$b"}},
	})
}
