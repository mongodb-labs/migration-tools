package testtools

import (
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type comparee[T any] interface {
	Compare(other T) int
}

var _ comparee[bson.Timestamp] = bson.Timestamp{}

// AssertCompareGreater asserts that a.Compare(b) > 0.
func AssertCompareGreater[T comparee[T]](t *testing.T, a, b T) {
	if a.Compare(b) <= 0 {
		t.Errorf("Expected %v to exceed %v", a, b)
	}
}

// AssertCompareGreaterOrEqual asserts that a.Compare(b) >= 0.
func AssertCompareGreaterOrEqual[T comparee[T]](t *testing.T, a, b T) {
	if a.Compare(b) < 0 {
		t.Errorf("Expected %v to equal or exceed %v", a, b)
	}
}
