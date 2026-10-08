package bsontools

import (
	"encoding/binary"
	"fmt"
	"slices"

	"github.com/ccoveille/go-safecast/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/x/bsonx/bsoncore"
)

// AppendInRaw appends a field named fieldName with the value newValue
// to the embedded document that pointer points to (or to the root
// document if pointer is empty).
//
// It returns an error if the pointer doesn’t resolve to an embedded
// document or if the target document already has a field named
// fieldName.
func AppendInRaw[T ~[]byte](
	raw T,
	fieldName string,
	newValue bson.RawValue,
	pointer ...string,
) (T, error) {
	sizeFromHeader, _, ok := bsoncore.ReadLength(raw)
	if !ok {
		return nil, fmt.Errorf("too few bytes to read BSON length")
	}

	if len(pointer) == 0 {
		return appendElementToRaw(raw, sizeFromHeader, fieldName, newValue)
	}

	return appendInSubDoc(raw, sizeFromHeader, fieldName, newValue, pointer)
}

// appendElementToRaw inserts the element into raw’s terminator
// position and updates raw’s length header.
func appendElementToRaw[T ~[]byte](
	raw T,
	sizeFromHeader int32,
	fieldName string,
	newValue bson.RawValue,
) (T, error) {
	if err := assertFieldIsAppendable(raw, sizeFromHeader, fieldName); err != nil {
		return nil, err
	}

	elementBytes := newValueBytes(fieldName, newValue)

	newRaw := slices.Insert(raw, int(sizeFromHeader)-1, elementBytes...)

	newSize, err := safecast.Convert[uint32](sizeFromHeader + int32(len(elementBytes)))
	if err != nil {
		return nil, err
	}
	binary.LittleEndian.PutUint32(newRaw, newSize)

	return newRaw, nil
}

// appendInSubDoc recurses into the embedded document or array that
// pointer’s first element names.
func appendInSubDoc[T ~[]byte](
	raw T,
	sizeFromHeader int32,
	fieldName string,
	newValue bson.RawValue,
	pointer []string,
) (T, error) {
	pos := 4
	for pos < int(sizeFromHeader)-1 {
		el, _, ok := bsoncore.ReadElement(raw[pos:])
		if !ok {
			return nil, fmt.Errorf("invalid BSON element at offset %d", pos)
		}

		keyBytes := el.KeyBytes()
		if string(keyBytes) != pointer[0] {
			pos += len(el)
			continue
		}

		return appendIntoElement(raw, fieldName, newValue, pointer, el, pos+1+len(keyBytes)+1)
	}

	return nil, fmt.Errorf("cannot append %#q: element not found", pointer)
}

// appendIntoElement appends to the document or array element at
// raw[valueAt], splices the result back into raw, and updates raw’s
// length header.
func appendIntoElement[T ~[]byte](
	raw T,
	fieldName string,
	newValue bson.RawValue,
	pointer []string,
	el bsoncore.Element,
	valueAt int,
) (T, error) {
	bsonType := bson.Type(el[0])

	if len(pointer) == 1 {
		if bsonType != bson.TypeEmbeddedDocument {
			return raw, fmt.Errorf(
				"cannot append %#q to %#q because it is of a simple type (%s)",
				fieldName,
				pointer[0],
				bsonType,
			)
		}
	} else if bsonType != bson.TypeArray && bsonType != bson.TypeEmbeddedDocument {
		return raw, PointerTooDeepError{
			givenPointer:   slices.Clone(pointer),
			elementType:    bsonType,
			elementPointer: slices.Clone(pointer[:1]),
		}
	}

	newSub, err := AppendInRaw(raw[valueAt:], fieldName, newValue, pointer[1:]...)
	if err != nil {
		return nil, err
	}

	// newSub is raw’s suffix with elementBytes inserted, so its length
	// difference is the number of inserted bytes.
	delta, err := safecast.Convert[int32](len(newSub) - len(raw[valueAt:]))
	if err != nil {
		return nil, err
	}

	sizeFromHeader, _, ok := bsoncore.ReadLength(raw)
	if !ok {
		return nil, fmt.Errorf("too few bytes to read BSON length")
	}

	newRaw := append(raw[:valueAt], newSub...)

	newSize, err := safecast.Convert[uint32](sizeFromHeader + delta)
	if err != nil {
		return nil, err
	}
	binary.LittleEndian.PutUint32(newRaw, newSize)

	return newRaw, nil
}

// assertFieldIsAppendable returns an error if the document in raw
// already contains a field named fieldName.
func assertFieldIsAppendable[T ~[]byte](raw T, sizeFromHeader int32, fieldName string) error {
	pos := 4
	for pos < int(sizeFromHeader)-1 {
		el, _, ok := bsoncore.ReadElement(raw[pos:])
		if !ok {
			return fmt.Errorf("invalid BSON element at offset %d", pos)
		}

		if string(el.KeyBytes()) == fieldName {
			return fmt.Errorf("document already has a field %#q", fieldName)
		}

		pos += len(el)
	}

	return nil
}

// newValueBytes encodes a complete BSON element (type byte, key
// cstring, and value) for fieldName and newValue.
func newValueBytes(fieldName string, newValue bson.RawValue) []byte {
	elementBytes := []byte{byte(newValue.Type)}
	elementBytes = append(elementBytes, fieldName...)
	elementBytes = append(elementBytes, 0x00)
	return append(elementBytes, newValue.Value...)
}
