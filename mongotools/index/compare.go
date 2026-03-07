package index

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"slices"

	"github.com/ccoveille/go-safecast/v2"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/mongodb-labs/migration-tools/bsontools"
	"github.com/samber/lo"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/x/bsonx/bsoncore"
)

// Index options that should be checked without ignoring the order of fields.
var optsWhereOrderIsSignificant = mapset.NewSet(
	"key",

	// NB: This is here in case any filters contain embedded documents, which
	// MongoDB compares order-sensitively.
	"partialFilterExpression",
)

// AreSpecsEqual compares two index specifications and returns a boolean
// that indicates whether they match. It:
// 1) normalizes legacy index specifications
// 2) omits the version field
// 2) correctly considers or ignores field order as appropriate.
func AreSpecsEqual(specA, specB bson.Raw) (bool, error) {
	specA = slices.Clone(specA)
	specB = slices.Clone(specB)

	specA, _, err := ModernizeSpec(specA)
	if err != nil {
		return false, err
	}
	specB, _, err = ModernizeSpec(specB)
	if err != nil {
		return false, err
	}

	specA, err = normalizeTypesInSpec(specA)
	if err != nil {
		return false, err
	}
	specB, err = normalizeTypesInSpec(specB)
	if err != nil {
		return false, err
	}

	// We can safely ignore the `v` field when comparing indexes on the source versus destination.
	// Mongosync doesn't support any server versions with v0 indexes: the server dropped support for
	// them when it dropped MMAPv1 support in 4.2 via SERVER-22987. There are no backwards-
	// incompatible features between v1 and v2 indexes. v2 indexes only added `NumberDecimal`
	// and `Collation`.
	specA, err = omitVersionFromIndexSpec(specA)
	if err != nil {
		return false, err
	}

	specB, err = omitVersionFromIndexSpec(specB)
	if err != nil {
		return false, err
	}

	sortedSpecA := slices.Clone(specA)
	if err := bsontools.SortFields(sortedSpecA); err != nil {
		return false, fmt.Errorf("sorting spec A’s fields: %w", err)
	}

	sortedSpecB := slices.Clone(specB)
	if err := bsontools.SortFields(sortedSpecB); err != nil {
		return false, fmt.Errorf("sorting spec A’s fields: %w", err)
	}

	if !bytes.Equal(sortedSpecA, sortedSpecB) {
		return false, nil
	}

	for optName := range optsWhereOrderIsSignificant.Iter() {
		var optValueA, optValueB bson.RawValue

		// NB: By this point we know the specs match when ignoring order.
		// That means that A & B have the same keys. Thus, we only need to
		// check existence in one of the specs.
		optValueA, err := specA.LookupErr(optName)
		if err != nil {
			if errors.Is(err, bsoncore.ErrElementNotFound) {
				continue
			}

			return false, fmt.Errorf("failed to look up %#q in spec A (%v): %w", optName, specA, err)
		}

		optValueB, err = specB.LookupErr(optName)
		if err != nil {
			if errors.Is(err, bsoncore.ErrElementNotFound) {
				continue
			}

			return false, fmt.Errorf("failed to look up %#q in spec B (%v): %w", optName, specB, err)
		}

		if !optValueA.Equal(optValueB) {
			return false, nil
		}
	}

	return true, nil
}

// The server stores certain index values differently from how they’re
// actually used. `expireAfterSeconds`, for example, gets stored as a
// double even though it’s always used as a long (i32). Thus, if you create
// an index with expireAfterSeconds=123.4, it’ll be used as 123, even
// though the server stores 123.4 internally.
//
// Some commands, such as `$indexStats` and `$listCatalog`, report internal
// values. That means it’s possible for two shards to have different
// “internal” values (e.g., 123.4 vs. 123) that actually get used the
// same way and, thus, don’t conflict in practice.
//
// The function below converts the stored/internal index properties to their
// “active” equivalents. This is the Go equivalent of NormalizeIndexDefinitionsStage().
// We decided to normalize indexes here as well as add it to our $listCatalog and $indexStats
// pipelines in case a future maintainer does not use our pre-defined pipelines to fetch indexes
// and compares them, in which case there may be a mismatch that would not exist had the indexes
// been normalized.
//
// For a table with all known type differences, see:
// https://github.com/10gen/mongo/blob/master/src/mongo/db/catalog/README.md#examples-of-differences-between-listindexes-and-listcatalog-results
func normalizeTypesInSpec(spec bson.Raw) (bson.Raw, error) {
	spec, err := convertTTLSpecToInt32(spec)
	if err != nil {
		return nil, err
	}

	spec, err = convertBitsSpecToInt32(spec)
	if err != nil {
		return nil, err
	}

	spec, err = convertSparseSpecToBool(spec)
	if err != nil {
		return nil, err
	}

	return spec, nil
}

// convertTTLSpecToInt32 ensures that TTL values are not greater than math.MaxInt32 and then converts the TTL to an int32.
// TTL values must be within 0 and math.MaxInt32 according to these docs:
// https://www.mongodb.com/docs/manual/core/index-ttl/#create-a-ttl-index. (Pre-5.0 versions do not enforce that TTl values
// are less than math.MaxInt32.) If the spec lacks a TTL value, the function returns without modifying the spec. This
// function returns an error if it sees an unexpected TTL value.
//
// This is a workaround for SERVER-91498 (a TTL value can be stored as an int, long, or decimal on the server). For example,
// if you create an index with a TTL value, the TTL is stored as an int32 on the server. If you collMod an index with a
// TTL value, the TTL is capped at MaxInt32 and stored as an int64 on the server.
func convertTTLSpecToInt32(spec bson.Raw) (bson.Raw, error) {
	return convertToInt32(spec, "expireAfterSeconds", math.MaxInt32)
}

// convertBitsSpecToInt32 is a workaround for SERVER-73442 where the server will accept a non-integer `bits` value during `createIndexes` even
// though it is stored as an int under the hood. It is safe to convert it to an int32 since `bits` can only take values of 1 to 32 inclusive.
// For more details, see: https://www.mongodb.com/docs/manual/core/indexes/index-types/geospatial/2d/create/define-location-precision/
func convertBitsSpecToInt32(spec bson.Raw) (bson.Raw, error) {
	return convertToInt32(spec, "bits", 32)
}

// convertToInt32 converts the value of the given key in the spec to an int32. It expects the original value of the key
// to be numeric. Additionally, this function should only be used for index spec values in the server that fit within a
// 32-bit signed integer range. This is because the server uses static_cast<int> in C++ to perform $toInt which can result
// in undefined behavior on overflow whereas Go does well-defined truncation when converting to int32. For more details, see:
// https://github.com/10gen/mongo/blob/84ff3493467477ffee5b92b663622c843d06fd9e/src/mongo/db/exec/expression/evaluate_math.cpp#L1158
func convertToInt32(spec bson.Raw, keyName string, maxBound int) (bson.Raw, error) {
	val, err := spec.LookupErr(keyName)
	if err != nil {
		if errors.Is(err, bsoncore.ErrElementNotFound) {
			return spec, nil
		}

		return nil, fmt.Errorf("extracting %#q: %w", keyName, err)
	}

	switch val.Type {
	case bson.TypeInt32:
		return spec, nil
	case bson.TypeInt64, bson.TypeDouble:
		if val.AsFloat64() > float64(maxBound) {
			return nil, fmt.Errorf(
				"%#q value (%d) cannot exceed %d",
				keyName,
				val.Int64(),
				maxBound,
			)
		}
	default:
		return nil, fmt.Errorf(
			"expected %#q to be numeric but got %s",
			keyName,
			val.Type,
		)
	}

	newSpec, found, err := bsontools.ReplaceInRaw(
		spec,
		bsontools.ToRawValue(safecast.MustConvert[int32](val.AsFloat64())),
		keyName,
	)

	lo.Assert(found, "must have found %#q", keyName)

	return newSpec, err
}

// Since the server allows numeric values for `sparse` but stores it as a bool, we normalize by converting all numeric
// `sparse` values to bools. This will error if the `sparse` value is non-numeric.
func convertSparseSpecToBool(spec bson.Raw) (bson.Raw, error) {
	keyName := "sparse"

	val, err := spec.LookupErr(keyName)
	if err != nil {
		if errors.Is(err, bsoncore.ErrElementNotFound) {
			return spec, nil
		}

		return nil, fmt.Errorf("extracting %#q: %w", keyName, err)
	}

	switch val.Type {
	case bson.TypeBoolean:
		return spec, nil
	case bson.TypeInt32, bson.TypeInt64, bson.TypeDouble:
	default:
		// The server does not allow non-numeric / bool types for `sparse` values.
		return nil, fmt.Errorf(
			"expected `sparse` to be a number or a bool; found %s",
			val.Type,
		)
	}

	newSpec, found, err := bsontools.ReplaceInRaw(
		spec,
		bsontools.ToRawValue(val.AsFloat64() != 0),
		keyName,
	)

	lo.Assert(found, "must have found %#q", keyName)

	return newSpec, err
}
