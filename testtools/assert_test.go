package testtools

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// failureCapturingT is a minimal [assert.TestingT] that records the
// failure output instead of failing the test.
type failureCapturingT struct {
	failed  bool
	message string
}

func (f *failureCapturingT) Errorf(format string, args ...any) {
	f.failed = true
	f.message = fmt.Sprintf(format, args...)
}

func TestAssertCompareGreater(t *testing.T) {
	timestamp1 := bson.Timestamp{T: 1, I: 1}
	timestamp2 := bson.Timestamp{T: 2, I: 2}

	AssertCompareGreater(t, timestamp2, timestamp1)
}

func TestAssertCompareGreater_Fails(t *testing.T) {
	timestamp1 := bson.Timestamp{T: 1, I: 1}
	timestamp2 := bson.Timestamp{T: 2, I: 2}

	mockT := &failureCapturingT{}
	AssertCompareGreater(mockT, timestamp1, timestamp2)

	assert.True(t, mockT.failed)
	assert.Contains(t, mockT.message, "must exceed")
	assert.Contains(t, mockT.message, fmt.Sprintf("%v", timestamp2))
	assert.Contains(t, mockT.message, fmt.Sprintf("%v", timestamp1))
}

func TestAssertCompareGreater_FailsWithCallerMessage(t *testing.T) {
	timestamp1 := bson.Timestamp{T: 1, I: 1}
	timestamp2 := bson.Timestamp{T: 2, I: 2}

	mockT := &failureCapturingT{}
	AssertCompareGreater(mockT, timestamp1, timestamp2, "expected %d", 42)

	assert.True(t, mockT.failed)
	assert.Contains(t, mockT.message, "must exceed")
	assert.Contains(t, mockT.message, "expected 42")
}

func TestAssertCompareGreaterOrEqual(t *testing.T) {
	timestamp1 := bson.Timestamp{T: 1, I: 1}
	timestamp2 := bson.Timestamp{T: 2, I: 2}

	AssertCompareGreaterOrEqual(t, timestamp2, timestamp2)
	AssertCompareGreaterOrEqual(t, timestamp2, timestamp1)
}

func TestAssertCompareGreaterOrEqual_Fails(t *testing.T) {
	timestamp1 := bson.Timestamp{T: 1, I: 1}
	timestamp2 := bson.Timestamp{T: 2, I: 2}

	mockT := &failureCapturingT{}
	AssertCompareGreaterOrEqual(mockT, timestamp1, timestamp2)

	assert.True(t, mockT.failed)
	assert.Contains(t, mockT.message, "must equal or exceed")
	assert.True(
		t,
		strings.Contains(mockT.message, fmt.Sprintf("%v", timestamp1)) &&
			strings.Contains(mockT.message, fmt.Sprintf("%v", timestamp2)),
	)
}
