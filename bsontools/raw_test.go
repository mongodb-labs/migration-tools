package bsontools

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/x/bsonx/bsoncore"
)

func TestRawLookup(t *testing.T) {
	doc := bson.D{
		{"foo", "bar"},
	}
	raw, err := bson.Marshal(doc)
	require.NoError(t, err)

	bar, err := RawLookup[string](raw, "foo")
	require.NoError(t, err)
	assert.Equal(t, "bar", bar)

	_, err = RawLookup[string](raw, "oops")
	assert.ErrorIs(t, err, bsoncore.ErrElementNotFound)

	_, err = RawLookup[string](raw, "foo", "bar")
	assert.ErrorAs(t, err, &bsoncore.InvalidDepthTraversalError{})

	_, err = RawLookup[int64](raw, "foo")
	assert.Error(t, err)
	assert.ErrorContains(t, err, "int64")
	assert.ErrorContains(t, err, "string")
}

// TestCountRawElementsNoAllocs verifies that CountRawElements does not
// heap-allocate when iterating a multi-field document. Regression here would
// indicate that one of the iterator's internal allocations leaked back in.
func TestCountRawElementsNoAllocs(t *testing.T) {
	doc := bson.D{
		{"s", "hello"},
		{"i32", int32(42)},
		{"i64", int64(1 << 40)},
		{"b", true},
		{"f", 3.14},
		{"sub", bson.D{{"x", "y"}}},
		{"arr", bson.A{1, 2, 3}},
	}
	raw, err := bson.Marshal(doc)
	require.NoError(t, err)

	// Sanity-check that we're actually counting all fields, so the no-alloc
	// loop below is exercising the real happy path.
	count, err := CountRawElements(raw)
	require.NoError(t, err)
	require.Equal(t, len(doc), count)

	avg := testing.AllocsPerRun(100, func() {
		_, err = CountRawElements(raw)
		require.NoError(t, err)
	})

	assert.Zero(t, avg, "CountRawElements should not heap-allocate")
}

// TestCountRawElementsEmptyBuffer verifies that CountRawElements treats an
// empty buffer as zero fields (with no error). Empty raw input is handled as
// an empty document, and CountRawElements returns zero for this case.
func TestCountRawElementsEmptyBuffer(t *testing.T) {
	count, err := CountRawElements(bson.Raw{})
	require.NoError(t, err)
	assert.Zero(t, count)

	count, err = CountRawElements(bson.Raw(nil))
	require.NoError(t, err)
	assert.Zero(t, count)
}

func Test_NoAlloc(t *testing.T) {
	raw, err := bson.Marshal(bson.D{
		{"foo", "bar"},
	})
	require.NoError(t, err)

	t.Run(
		"CountRawElements",
		func(t *testing.T) {
			avg := testing.AllocsPerRun(100, func() {
				count, err := CountRawElements(raw)
				require.NoError(t, err)
				assert.Equal(t, 1, count)
			})

			assert.Zero(t, avg, "should not allocate")
		},
	)

	t.Run(
		"RawElements",
		func(t *testing.T) {
			els := make([]bson.RawElement, 0, 1)

			avg := testing.AllocsPerRun(1000, func() {
				els = els[:0]
				for el, err := range RawElements(raw) {
					require.NoError(t, err)
					els = append(els, el)
				}
			})

			assert.Zero(t, avg, "should not allocate")
		},
	)
}

func TestRawElements_Empty(t *testing.T) {
	var doc bson.Raw

	for _, err := range RawElements(doc) {
		require.NoError(t, err)
		require.Fail(t, "should have no elements")
	}

	doc = bson.Raw{}

	for _, err := range RawElements(doc) {
		require.NoError(t, err)
		require.Fail(t, "should have no elements")
	}
}

// TestRawElements_FiveByteAllNUL verifies that the canonical empty BSON
// document (length=5, then a single 0x00 terminator) is treated identically
// to an empty buffer: no error, no elements yielded.
func TestRawElements_FiveByteAllNUL(t *testing.T) {
	doc := bson.Raw{0x05, 0, 0, 0, 0}

	for _, err := range RawElements(doc) {
		require.NoError(t, err)
		require.Fail(t, "should have no elements")
	}
}

// TestRawElements_LengthMismatch verifies that a buffer whose declared length
// does not match its actual length is rejected. Without this check, a
// truncated buffer can yield a partial run of valid-looking elements before
// failing (or worse, silently produce garbage on length-extended buffers).
func TestRawElements_LengthMismatch(t *testing.T) {
	raw, err := bson.Marshal(bson.D{{"foo", "bar"}})
	require.NoError(t, err)

	t.Run("buffer truncated", func(t *testing.T) {
		var iterErr error
		for _, err := range RawElements(raw[:len(raw)-1]) {
			if err != nil {
				iterErr = err
				break
			}
		}
		assert.Error(t, iterErr, "truncated buffer should produce an error")
	})

	t.Run("buffer extended", func(t *testing.T) {
		extended := append([]byte{}, raw...)
		extended = append(extended, 0x00)
		var iterErr error
		for _, err := range RawElements(extended) {
			if err != nil {
				iterErr = err
				break
			}
		}
		assert.Error(t, iterErr, "extended buffer should produce an error")
	})
}

// TestRawElements_BelowMinimumLength verifies that a non-empty buffer whose
// declared length is below the BSON minimum (5 bytes) is rejected.
func TestRawElements_BelowMinimumLength(t *testing.T) {
	// 4-byte buffer self-consistently claiming length=4: header is correct,
	// length matches buffer, but no room for the required terminator.
	doc := []byte{0x04, 0x00, 0x00, 0x00}

	var iterErr error
	for _, err := range RawElements(doc) {
		if err != nil {
			iterErr = err
			break
		}
	}
	assert.Error(t, iterErr, "below-minimum-length doc should produce an error")
}

// TestRawElements_MissingTerminator verifies that a doc whose final byte is
// not the required 0x00 is rejected.
func TestRawElements_MissingTerminator(t *testing.T) {
	// 5-byte buffer claiming length=5 but with a non-NUL final byte.
	doc := []byte{0x05, 0x00, 0x00, 0x00, 0xFF}

	var iterErr error
	for _, err := range RawElements(doc) {
		if err != nil {
			iterErr = err
			break
		}
	}
	assert.Error(t, iterErr, "missing-terminator doc should produce an error")
}

// TestRawElements_ElementConsumesTerminator verifies that a malformed last
// element whose declared length would consume the document's trailing 0x00
// terminator is detected (rather than silently swallowing the terminator).
func TestRawElements_ElementConsumesTerminator(t *testing.T) {
	// {"a": "ab"} marshals as: [len=15][type=02][key=a\0][strlen=3][a b \0][doc-term=0]
	// Bump the inner string-length to 4 so the value claim consumes the
	// document terminator.
	doc, err := bson.Marshal(bson.D{{"a", "ab"}})
	require.NoError(t, err)

	idx := bytes.Index(doc, []byte("ab")) - 4
	require.GreaterOrEqual(t, idx, 0)
	binary.LittleEndian.PutUint32(doc[idx:idx+4], 4)

	var iterErr error
	for _, err := range RawElements(doc) {
		if err != nil {
			iterErr = err
			break
		}
	}
	assert.Error(t, iterErr,
		"element-consumes-terminator should produce an error, "+
			"not silently yield a corrupted element")
}

func TestRawElements_ShortHeader(t *testing.T) {
	doc := bson.Raw{0, 1, 2}

	for _, err := range RawElements(doc) {
		require.Error(t, err)
		assert.ErrorAs(t, err, &bsoncore.InsufficientBytesError{})
		break // Must stop iterating after an error or RawElements panics.
	}
}

func TestRawElements(t *testing.T) {
	srcD := bson.D{
		{"foo", "xxx"},
		{"bar", "baz"},
	}

	mydoc, err := bson.Marshal(srcD)
	require.NoError(t, err)

	count, err := CountRawElements(mydoc)
	require.NoError(t, err)

	assert.Equal(t, len(srcD), count)

	received := bson.D{}

	for el, err := range RawElements(mydoc) {
		require.NoError(t, err, "should iterate")

		received = append(received, bson.E{
			lo.Must(el.KeyErr()),
			lo.Must(el.ValueErr()).StringValue(),
		})
	}

	assert.Equal(t, srcD, received, "should iterate all fields")

	// Now make the document invalid
	mydoc = mydoc[:len(mydoc)-3]

	received = received[:0]

	var iterErr error
	for _, err := range RawElements(mydoc) {
		if err != nil {
			iterErr = err
			break
		}
	}

	assert.Error(t, iterErr, "should fail somewhere in the iteration")

	assert.Panics(
		t,
		func() {
			var gotErr error
			for _, err := range RawElements(mydoc) {
				if err != nil {
					gotErr = err
				}
			}

			assert.Error(t, gotErr, "should get an error")
		},
		"should panic if we fail but didn’t stop iterating",
	)
}
