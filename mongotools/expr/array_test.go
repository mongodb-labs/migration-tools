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
