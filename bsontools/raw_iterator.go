package bsontools

import (
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/x/bsonx/bsoncore"
)

// RawIterator is an iterator over a bson.Raw’s elements.
//
// Use it like:
//
//	iter, err := NewRawIterator(doc)
//	if err != nil { ... }
//	for el := iter.Next(); el != nil; el = iter.Next() {
//	    // use el
//	}
//	if err := iter.Err(); err != nil { ... }
type RawIterator struct {
	orig       []byte
	remaining  []byte
	fieldIndex int
	err        error
}

// NewRawIterator returns a new RawIterator over the given BSON document.
// Note that this returns a struct literal rather than a pointer so as to
// avoid an unneeded heap allocation.
//
// An empty buffer is treated as a valid empty BSON document (equivalent to
// the 5-byte all-NUL form): no error, no elements yielded.
//
// If the document is non-empty but too short to contain a BSON length header,
// or if the declared document length mismatches the buffer length, an error
// is returned.
func NewRawIterator[D ~[]byte](doc D) (RawIterator, error) {
	if len(doc) == 0 {
		return RawIterator{}, nil
	}

	length, rem, ok := bsoncore.ReadLength(doc)

	if !ok {
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

	return RawIterator{
		orig:      doc,
		remaining: doc[4:],
	}, nil
}

// ParsedFields returns the # of fields returned by the iterator so far.
func (ri *RawIterator) ParsedFields() int {
	return ri.fieldIndex
}

// Next returns the next element in the iteration, or nil at end-of-document
// or after an error. Use Err to distinguish those two cases. Once an error
// is set or the document is exhausted, subsequent calls keep returning nil.
func (ri *RawIterator) Next() bson.RawElement {
	if ri.err != nil || len(ri.remaining) <= 1 {
		return nil
	}

	el, rem, ok := bsoncore.ReadElement(ri.remaining)

	if !ok {
		ri.err = bsoncore.NewInsufficientBytesError(ri.orig, rem)
		return nil
	}

	if err := el.Validate(); err != nil {
		ri.err = err
		return nil
	}

	ri.fieldIndex++
	ri.remaining = rem

	return bson.RawElement(el)
}

// Err returns the last error encountered by Next, or nil if iteration ended
// cleanly (end-of-document) or has not yet errored.
func (ri *RawIterator) Err() error {
	return ri.err
}
