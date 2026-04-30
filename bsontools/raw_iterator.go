package bsontools

import (
	"fmt"

	"github.com/mongodb-labs/migration-tools/option"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/x/bsonx/bsoncore"
)

// RawIterator is an iterator over a bson.Raw’s elements.
type RawIterator struct {
	remaining  []byte
	fieldIndex int
}

// NewRawIterator returns a new RawIterator over the given BSON document.
// Note that this returns a struct literal rather than a pointer so as to
// avoid an unneeded heap allocation.
//
// If the document is too short to contain a BSON document, or if the declared
// document length mismatches the buffer length, an error is returned.
func NewRawIterator[D ~[]byte](doc D) (RawIterator, error) {
	length, rem, ok := bsoncore.ReadLength(doc)

	if !ok {
		if len(doc) == 0 {
			return RawIterator{}, fmt.Errorf(
				"%w: BSON document is empty",
				bsoncore.NewInsufficientBytesError(doc, rem),
			)
		}

		return RawIterator{}, fmt.Errorf(
			"%w (buffer is only %d bytes long)",
			bsoncore.NewInsufficientBytesError(doc, rem),
			len(doc),
		)
	}

	if int(length) != len(doc) {
		return RawIterator{}, fmt.Errorf(
			"declared document length (%d) mismatches actual buffer length (%d)",
			length,
			len(doc),
		)
	}

	return RawIterator{remaining: doc[4:]}, nil
}

// FieldIndex returns the next-parsed field’s 0-based index.
func (ri *RawIterator) FieldIndex() int {
	return ri.fieldIndex
}

// Next returns the next element in the iteration, or None if there are no more elements.
func (ri *RawIterator) Next() (option.Option[bson.RawElement], error) {
	if len(ri.remaining) == 0 {
		return option.None[bson.RawElement](), nil
	}

	if len(ri.remaining) == 1 {
		if ri.remaining[0] != 0x00 {
			return option.None[bson.RawElement](), fmt.Errorf(
				"invalid BSON document terminator: got 0x%02x, want 0x00",
				ri.remaining[0],
			)
		}

		return option.None[bson.RawElement](), nil
	}

	el, rem, ok := bsoncore.ReadElement(ri.remaining)

	if !ok {
		return option.None[bson.RawElement](), bsoncore.NewInsufficientBytesError(ri.remaining, rem)
	}

	if err := el.Validate(); err != nil {
		return option.None[bson.RawElement](), err
	}

	ri.fieldIndex++

	ri.remaining = rem

	return option.Some(bson.RawElement(el)), nil
}
