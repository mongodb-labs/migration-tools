package testtools

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type comparee[T any] interface {
	Compare(other T) int
}

var _ comparee[bson.Timestamp] = bson.Timestamp{}

// AssertCompareGreater asserts that a.Compare(b) > 0.
func AssertCompareGreater[T comparee[T]](
	t *testing.T,
	a, b T,
	msgAndArgs ...any,
) {
	t.Helper()

	if a.Compare(b) <= 0 {
		assert.Fail(
			t,
			fmt.Sprintf("%v must exceed %v", a, b),
			msgAndArgs...,
		)
	}
}

// AssertCompareGreaterOrEqual asserts that a.Compare(b) >= 0.
func AssertCompareGreaterOrEqual[T comparee[T]](
	t *testing.T,
	a, b T,
	msgAndArgs ...any,
) {
	t.Helper()

	if a.Compare(b) < 0 {
		assert.Fail(
			t,
			fmt.Sprintf("%v must equal or exceed %v", a, b),
			msgAndArgs...,
		)
	}
}
