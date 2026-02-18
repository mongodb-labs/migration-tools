package mongotools

import (
	"bytes"
	"fmt"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/mongodb-labs/migration-tools/bsontools"
	"go.mongodb.org/mongo-driver/v2/bson"
)

var typeComparator = map[bson.Type]func(a, b bson.RawValue) (int, error){
	bson.TypeInt64:  bsontools.CompareInt64s,
	bson.TypeString: compareStringRecordID,
	bson.TypeBinary: compareBinaryRecordID,
}

// GetKnownRecordIDTypes returns all known record ID types.
func GetKnownRecordIDTypes() mapset.Set[bson.Type] {
	return mapset.NewSetFromMapKeys(typeComparator)
}

// CompareRecordIDs compares two BSON record IDs.
//
// For the most part these obey the same rules as normal
// [BSON sorting]. The one exception is binary strings: here, instead of
// sorting length-first, we sort the buffers in standard binary order.
//
// [BSON sorting]: https://www.mongodb.com/docs/manual/reference/bson-type-comparison-order/
func CompareRecordIDs(a, b bson.RawValue) (int, error) {
	comparator, ok := typeComparator[a.Type]
	if !ok {
		return 0, createCannotCompareTypesErr(a, b)
	}

	return comparator(a, b)
}

func compareStringRecordID(a, b bson.RawValue) (int, error) {
	// A v5 time-series collection might be upgraded in-place. In this case
	// the v5-era string record IDs would coexist with newer, binary ones.
	switch b.Type {
	case bson.TypeString:
		return bsontools.CompareStrings(a, b)
	case bson.TypeBinary:
		return -1, nil
	default:
		return 0, createCannotCompareTypesErr(a, b)
	}
}

func compareBinaryRecordID(a, b bson.RawValue) (int, error) {
	switch b.Type {
	case bson.TypeString:
		return 1, nil
	case bson.TypeBinary:
		// See below.
	default:
		return 0, createCannotCompareTypesErr(a, b)
	}

	aBin, err := bsontools.RawValueToBinary(a)
	if err != nil {
		return 0, err
	}

	bBin, err := bsontools.RawValueToBinary(b)
	if err != nil {
		return 0, err
	}

	if aBin.Subtype != bBin.Subtype {
		return 0, fmt.Errorf("cannot compare BSON binary subtypes %d and %d", aBin.Subtype, bBin.Subtype)
	}

	return bytes.Compare(aBin.Data, bBin.Data), nil
}

func createCannotCompareTypesErr(a, b bson.RawValue) error {
	return fmt.Errorf("cannot compare BSON %s and %s record IDs", a.Type, b.Type)
}
