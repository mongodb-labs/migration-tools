package bsontools

import (
	"fmt"
	"iter"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/x/bsonx/bsoncore"
)

// RawLookup extracts & unmarshals a referent value from a BSON document.
// It’s like bson.Raw.LookupErr combined with RawValueTo.
func RawLookup[T unmarshalTargets, D ~[]byte](in D, pointer ...string) (T, error) {
	doc := bson.Raw(in)

	rv, err := doc.LookupErr(pointer...)
	if err != nil {
		return *new(T), fmt.Errorf("extracting %#q: %w", pointer, err)
	}

	val, err := RawValueTo[T](rv)
	if err != nil {
		return *new(T), fmt.Errorf("casting %#q: %w", pointer, err)
	}

	return val, err
}

// CountRawElements returns a count of the fields in the given BSON document.
// An empty buffer is treated as zero fields (no error).
func CountRawElements[D ~[]byte](doc D) (int, error) {
	rawIter, err := NewRawIterator(doc)
	if err != nil {
		return 0, err
	}

	count := 0

	for el := rawIter.Next(); el != nil; el = rawIter.Next() {
		count++
	}

	if err := rawIter.Err(); err != nil {
		return 0, err
	}

	return count, nil
}

// RawElements returns an iterator over a Raw’s elements. If the given slice
// is empty, the iterator yields no elements.
//
// If the iterator returns an error but the caller continues iterating,
// a panic will ensue.
//
// NB: Consider RawIterator instead in hot code paths, since it avoids
// heap-allocating closures. Like this helper, it treats an empty buffer as an
// empty document.
func RawElements[D ~[]byte](doc D) iter.Seq2[bson.RawElement, error] {
	if len(doc) == 0 {
		return func(func(bson.RawElement, error) bool) {}
	}

	if _, rem, ok := bsoncore.ReadLength(doc); !ok {
		return func(yield func(bson.RawElement, error) bool) {
			yield(nil, fmt.Errorf(
				"%w (buffer is only %d bytes long)",
				bsoncore.NewInsufficientBytesError(doc, rem),
				len(doc),
			))
		}
	}

	remaining := doc[4:]

	return func(yield func(bson.RawElement, error) bool) {
		var el bsoncore.Element
		var ok bool

		for len(remaining) > 1 {
			el, remaining, ok = bsoncore.ReadElement(remaining)

			var err error

			if !ok {
				err = bsoncore.NewInsufficientBytesError(doc, remaining)
			} else {
				err = el.Validate()
			}

			if err != nil {
				if yield(nil, err) {
					panic(fmt.Sprintf("Must stop iteration after error (%v)", err))
				}

				return
			}

			if !yield(bson.RawElement(el), nil) {
				return
			}
		}
	}
}
