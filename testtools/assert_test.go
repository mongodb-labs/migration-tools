package testtools

import (
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestAssertCompareGreater(t *testing.T) {
	timestamp1 := bson.Timestamp{T: 1, I: 1}
	timestamp2 := bson.Timestamp{T: 2, I: 2}

	AssertCompareGreater(t, timestamp2, timestamp1)
}

func TestAssertCompareGreaterOrEqual(t *testing.T) {
	timestamp1 := bson.Timestamp{T: 1, I: 1}
	timestamp2 := bson.Timestamp{T: 2, I: 2}

	AssertCompareGreaterOrEqual(t, timestamp2, timestamp2)
	AssertCompareGreaterOrEqual(t, timestamp2, timestamp1)
}
