package bsontools

import (
	"fmt"

	"github.com/mongodb-labs/migration-tools/option"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/x/bsonx/bsoncore"
)

type RawIterator struct {
	remaining  []byte
	fieldIndex int
}

func NewRawIterator[D ~[]byte](doc D) (RawIterator, error) {
	if len(doc) == 0 {
		return RawIterator{}, nil
	}

	if _, rem, ok := bsoncore.ReadLength(doc); !ok {
		return RawIterator{}, fmt.Errorf(
			"%w (buffer is only %d bytes long)",
			bsoncore.NewInsufficientBytesError(doc, rem),
			len(doc),
		)
	}

	return RawIterator{remaining: doc[4:]}, nil
}

func (ri *RawIterator) FieldIndex() int {
	return ri.fieldIndex
}

func (ri *RawIterator) Next() (option.Option[bson.RawElement], error) {
	if len(ri.remaining) <= 1 {
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
