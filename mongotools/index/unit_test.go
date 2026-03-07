package index

import (
	"fmt"
	"math"
	"slices"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/suite"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type UnitTestSuite struct {
	suite.Suite
}

func TestUnitTestSuite(t *testing.T) {
	ts := new(UnitTestSuite)
	suite.Run(t, ts)
}

const (
	expireAfterSecondsKey = "expireAfterSeconds"
	bitsKey               = "bits"
	sparseKey             = "sparse"
)

func (s *UnitTestSuite) TestDescribeDiffs() {
	cases := []struct {
		a, b       bson.D
		diffPieces []string
		label      string
	}{
		{
			a: bson.D{
				{"v", 2},
				{"key", bson.D{{"a", 1}}},
			},
			b: bson.D{
				{"v", 1},
				{"key", bson.D{{"a", 1}}},
				{"sparse", true},
			},
			label:      "sparse",
			diffPieces: []string{"/sparse"},
		},
		{
			a: bson.D{
				{"v", 2},
				{"key", bson.D{{"a", 1}, {"b", 1}}},
			},
			b: bson.D{
				{"v", 1},
				{"key", bson.D{{"b", 1}, {"a", 1}}},
			},
			label:      "sparse",
			diffPieces: []string{"key"},
		},
	}

	for _, curCase := range cases {
		s.Run(
			curCase.label,
			func() {
				a, err := bson.Marshal(curCase.a)
				s.Require().NoError(err)

				b, err := bson.Marshal(curCase.b)
				s.Require().NoError(err)

				diff, err := DescribeSpecDifferences(a, b)
				s.Require().NoError(err)

				s.Require().NotZero(diff)

				for _, piece := range curCase.diffPieces {
					s.Assert().Contains(diff.MustGet(), piece)
				}
			},
		)
	}
}

func (s *UnitTestSuite) TestSameSpec() {
	cases := []struct {
		a, b  bson.D
		label string
	}{
		{
			a: bson.D{
				{"v", 2},
				{"key", bson.D{{"a", 1}}},
			},
			b: bson.D{
				{"v", 1},
				{"key", bson.D{{"a", 1}}},
				{"background", true},
			},
			label: "background",
		},
		{
			a: bson.D{
				{"v", 2},
				{"key", bson.D{{"a", 1}}},
			},
			b: bson.D{
				{"v", 1},
				{"key", bson.D{{"a", 1}}},
				{"ns", "foo.bar"},
			},
			label: "ns",
		},
		{
			a: bson.D{
				{"v", 2},
				{"key", bson.D{{"a", 1}}},
				{"sparse", true},
			},
			b: bson.D{
				{"v", 1},
				{"sparse", 1},
				{"key", bson.D{{"a", 1}}},
			},
			label: "spec field order; sparse type",
		},
	}

	for _, curCase := range cases {
		s.Run(
			curCase.label,
			func() {
				a, err := bson.Marshal(curCase.a)
				s.Require().NoError(err)

				b, err := bson.Marshal(curCase.b)
				s.Require().NoError(err)

				diff, err := DescribeSpecDifferences(a, b)
				s.Require().NoError(err)

				s.Assert().Zero(diff)
			},
		)
	}
}

//nolint:funlen
func (s *UnitTestSuite) Test_ConvertLegacyIndexKeys() {
	type testCase struct {
		name                string
		legacyKey           bson.D
		convertedKey        bson.D
		expectModernization bool
	}
	cases := []testCase{
		{
			name: "0",
			legacyKey: bson.D{
				{"a", 0},
				{"b", int32(0)},
				{"c", int64(0)},
				{"d", float32(0)},
				{"d", float64(0)},
				{"int32field", 1},
				{"int32field", int32(2)},
				{"int64field", int64(-3)},
				{"float64field", float32(-1)},
				{"float64field", float32(-1.1)},
				{"float64field", float64(-1)},
				{"float64field", float64(-1.1)},
			},
			convertedKey: bson.D{
				{"a", int32(1)},
				{"b", int32(1)},
				{"c", int32(1)},
				{"d", int32(1)},
				{"d", int32(1)},
				{"int32field", 1},
				{"int32field", int32(2)},
				{"int64field", int64(-3)},
				{"float64field", float32(-1)},
				{"float64field", float32(-1.1)},
				{"float64field", float64(-1)},
				{"float64field", float64(-1.1)},
			},
			expectModernization: true,
		},
		{
			name: "really small negative float values",
			legacyKey: bson.D{
				{"a", float64(-1e-12)},
			},
			convertedKey: bson.D{
				{"a", float64(-1e-12)},
			},
			expectModernization: false,
		},
		{
			name: "really small positive float values",
			legacyKey: bson.D{
				{"a", float64(1e-12)},
			},
			convertedKey: bson.D{
				{"a", float64(1e-12)},
			},
			expectModernization: false,
		},
		{
			name: "all 1's",
			legacyKey: bson.D{
				{"a", 1},
				{"b", int32(1)},
				{"c", int64(1)},
				{"d", float32(1)},
				{"d", float64(1)},
			},
			convertedKey: bson.D{
				{"a", 1},
				{"b", int32(1)},
				{"c", int64(1)},
				{"d", float32(1)},
				{"d", float64(1)},
			},
			expectModernization: false,
		},
		{
			name: "-0's",
			legacyKey: bson.D{
				{"a", -0},
				{"b", int32(-0)},
				{"c", int64(-0)},
			},
			convertedKey: bson.D{
				{"a", int32(1)},
				{"b", int32(1)},
				{"c", int32(1)},
			},
			expectModernization: true,
		},
		{
			name: "empty string",
			legacyKey: bson.D{
				{"key1", ""},
				{"key2", "2dsphere"},
			},
			convertedKey: bson.D{
				{"key1", int32(1)},
				{"key2", "2dsphere"},
			},
			expectModernization: true,
		},
		{
			name:                "bson.E",
			legacyKey:           bson.D{{"key1", bson.D{{"invalid", 1}}}},
			convertedKey:        bson.D{{"key1", int32(1)}},
			expectModernization: true,
		},
		{
			name:                "binary",
			legacyKey:           bson.D{{"key1", bson.Binary{}}},
			convertedKey:        bson.D{{"key1", int32(1)}},
			expectModernization: true,
		},
		{
			name:                "array",
			legacyKey:           bson.D{{"key1", bson.A{1, 2, 3}}},
			convertedKey:        bson.D{{"key1", int32(1)}},
			expectModernization: true,
		},
		{
			name: "min key and max key",
			legacyKey: bson.D{
				{"key1", bson.MinKey{}},
				{"key2", bson.MaxKey{}},
			},
			convertedKey: bson.D{
				{"key1", int32(1)},
				{"key2", int32(1)},
			},
			expectModernization: true,
		},
		{
			name:                "null",
			legacyKey:           bson.D{{"key1", bson.Null{}}},
			convertedKey:        bson.D{{"key1", int32(1)}},
			expectModernization: true,
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.testIndexKeyConversions(tc.legacyKey, tc.convertedKey, tc.expectModernization)
		})
	}
}

func (s *UnitTestSuite) Test_ConvertLegacyIndexKeys_Decimal0() {
	decimalNOne, err := bson.ParseDecimal128("-1")
	s.Require().NoError(err)
	decimalZero, err := bson.ParseDecimal128("0")
	s.Require().NoError(err)
	decimalOne, err := bson.ParseDecimal128("1")
	s.Require().NoError(err)
	decimalZero1, err := bson.ParseDecimal128("0.00")
	s.Require().NoError(err)

	legacyKey := bson.D{
		{"key1", decimalNOne},
		{"key2", decimalZero},
		{"key3", decimalOne},
		{"key4", decimalZero1},
	}
	convertedKey := bson.D{
		{"key1", decimalNOne},
		{"key2", int32(1)},
		{"key3", decimalOne},
		{"key4", int32(1)},
	}
	s.testIndexKeyConversions(legacyKey, convertedKey, true)
}

func (s *UnitTestSuite) TestDontConvertModernIndexes() {
	index := bson.D{
		{"v", 2},
		{"name", "indecks"},
		{"key", bson.D{{"a", 0}}},
	}

	rawIndex, err := bson.Marshal(index)
	s.Require().NoError(err)

	_, didModernize, err := ModernizeSpec(rawIndex)
	s.Require().NoError(err)
	s.Assert().False(didModernize, "should not modernize v2+ indexes")
}

func (s *UnitTestSuite) TestErrorOnUnsupportedIndexVersions() {
	for _, v := range []int{0, 3} {
		s.Run(fmt.Sprintf("version %d", v), func() {
			index := bson.D{
				{"v", v},
				{"name", "indecks"},
				{"key", bson.D{{"a", 0}}},
			}

			rawIndex, err := bson.Marshal(index)
			s.Require().NoError(err)

			_, didModernize, err := ModernizeSpec(rawIndex)
			s.Require().Error(err, "should error if not v1 or v2 index")
			s.Assert().ErrorContains(err, "unexpected")
			s.Assert().False(didModernize)
		})
	}
}

func (s *UnitTestSuite) TestOmitVersionFromIndexSpec() {
	s.Run("no version", func() {
		spec := bson.D{
			{"name", "indecks"},
			{"key", bson.D{{"a", 1}}},
		}

		rawSpec, err := bson.Marshal(spec)
		s.Require().NoError(err)

		origSpec := slices.Clone(rawSpec)

		rawSpec, err = omitVersionFromIndexSpec(rawSpec)
		s.Require().NoError(err)
		s.Assert().Equal(
			origSpec,
			rawSpec,
		)
	})

	s.Run("v: 2", func() {
		spec := bson.D{
			{"name", "indecks"},
			{"key", bson.D{{"a", 1}}},
			{"v", 2},
		}

		rawSpec, err := bson.Marshal(spec)
		s.Require().NoError(err)

		rawSpec, err = omitVersionFromIndexSpec(rawSpec)
		s.Require().NoError(err)

		s.Assert().Equal(
			lo.Must(bson.Marshal(bson.D{
				{"name", "indecks"},
				{"key", bson.D{{"a", 1}}},
			})),
			rawSpec,
		)
	})

	s.Run("v: something", func() {
		spec := bson.D{
			{"name", "indecks"},
			{"key", bson.D{{"a", 1}}},
			{"v", "something"},
		}

		rawSpec, err := bson.Marshal(spec)
		s.Require().NoError(err)

		rawSpec, err = omitVersionFromIndexSpec(rawSpec)
		s.Require().NoError(err)

		rawSpec, err = omitVersionFromIndexSpec(rawSpec)
		s.Require().NoError(err)

		s.Assert().Equal(
			lo.Must(bson.Marshal(bson.D{
				{"name", "indecks"},
				{"key", bson.D{{"a", 1}}},
			})),
			rawSpec,
		)
	})
}

func getLegacyIndexAndConvertedIndex(
	legacyKey bson.D,
	convertedKey bson.D,
) (bson.Raw, bson.Raw) {
	return getSpec(legacyKey), getSpec(convertedKey)
}

func getSpec(
	key bson.D,
) bson.Raw {
	return lo.Must(bson.Marshal(bson.D{
		{"v", int32(1)},
		{"key", key},
		{"name", "a_1"},
	}))
}

func (s *UnitTestSuite) TestConvertTTLSpecToInt32() {
	spec := lo.Must(bson.Marshal(bson.D{
		{"expireAfterSeconds", "a"},
	}))
	_, err := convertTTLSpecToInt32(spec)
	s.Assert().Error(err, "should error when TTL is not a number")

	spec = lo.Must(bson.Marshal(bson.D{
		{"expireAfterSeconds", int64(1 + math.MaxInt32)},
	}))
	_, err = convertTTLSpecToInt32(spec)
	s.Assert().Error(err, "should error when TTL is greater than math.MaxInt32")

	spec = lo.Must(bson.Marshal(bson.D{
		{"expireAfterSeconds", float64(math.MinInt32 - 1)},
	}))
	_, err = convertTTLSpecToInt32(spec)
	s.Assert().Error(err, "should error when TTL is less than math.MinInt32")

	spec = lo.Must(bson.Marshal(bson.D{{"unique", true}}))
	spec, err = convertTTLSpecToInt32(spec)
	s.Require().NoError(err)

	s.Require().Equal(
		lo.Must(bson.Marshal(bson.D{{"unique", true}})),
		spec,
		"spec without TTL should be unchanged",
	)

	spec = lo.Must(bson.Marshal(bson.D{{"expireAfterSeconds", int64(1000)}}))
	spec, err = convertTTLSpecToInt32(spec)
	s.Require().NoError(err)

	s.Assert().Equal(
		lo.Must(bson.Marshal(bson.D{{"expireAfterSeconds", int32(1000)}})),
		spec,
		"int64 TTL should be converted to int32",
	)

	spec = lo.Must(bson.Marshal(bson.D{{"expireAfterSeconds", float32(1000)}}))
	spec, err = convertTTLSpecToInt32(spec)
	s.Require().NoError(err)

	s.Assert().Equal(
		lo.Must(bson.Marshal(bson.D{{"expireAfterSeconds", int32(1000)}})),
		spec,
		"float32 TTL should be converted to int32",
	)

	spec = lo.Must(bson.Marshal(bson.D{{"expireAfterSeconds", float64(1000)}}))
	spec, err = convertTTLSpecToInt32(spec)
	s.Require().NoError(err)

	s.Assert().Equal(
		lo.Must(bson.Marshal(bson.D{{"expireAfterSeconds", int32(1000)}})),
		spec,
		"float64 TTL should be converted to int32",
	)
}

// TestConvertBitsSpecToInt32 only tests some `bits` conversions since
// TestConvertTTLSpecToInt32 already tests a lot of edge cases when converting
// a spec to int32.
func (s *UnitTestSuite) TestConvertBitsSpecToInt32() {
	spec := lo.Must(bson.Marshal(bson.D{
		{"bits", "a"},
	}))
	_, err := convertBitsSpecToInt32(spec)
	s.Assert().Error(err, "should error when bits is not a number")

	spec = lo.Must(bson.Marshal(bson.D{{"unique", true}}))
	spec, err = convertTTLSpecToInt32(spec)
	s.Require().NoError(err)

	s.Require().Equal(
		lo.Must(bson.Marshal(bson.D{{"unique", true}})),
		spec,
		"spec without bits should be unchanged",
	)

	spec = lo.Must(bson.Marshal(bson.D{
		{bitsKey, int64(43)},
	}))
	_, err = convertBitsSpecToInt32(spec)
	s.Assert().Error(err, "should error when bits is greater than 32")

	spec = lo.Must(bson.Marshal(bson.D{{bitsKey, float32(24)}}))

	spec, err = convertBitsSpecToInt32(spec)
	s.Require().NoError(err)

	s.Assert().Equal(
		lo.Must(bson.Marshal(bson.D{{bitsKey, int32(24)}})),
		spec,
		"float32 bits should be converted to int32",
	)
}

func (s *UnitTestSuite) TestConvertSparseSpecToBool() {
	spec := lo.Must(bson.Marshal(bson.D{
		{sparseKey, "a"},
	}))
	_, err := convertSparseSpecToBool(spec)
	s.Require().Error(err, "should error when sparse is not a number or a bool")

	spec = lo.Must(bson.Marshal(bson.D{{"unique", true}}))
	spec, err = convertSparseSpecToBool(spec)
	s.Require().NoError(err)

	s.Require().Equal(
		lo.Must(bson.Marshal(bson.D{{"unique", true}})),
		spec,
		"spec without sparse should be unchanged",
	)

	spec = lo.Must(bson.Marshal(bson.D{{sparseKey, int64(1234)}}))
	spec, err = convertSparseSpecToBool(spec)
	s.Require().NoError(err)

	s.Assert().Equal(
		lo.Must(bson.Marshal(bson.D{{sparseKey, true}})),
		spec,
		"positive int64 sparse value should be converted to true",
	)

	spec = lo.Must(bson.Marshal(bson.D{{sparseKey, 0}}))
	spec, err = convertSparseSpecToBool(spec)
	s.Require().NoError(err)

	s.Assert().Equal(
		lo.Must(bson.Marshal(bson.D{{sparseKey, false}})),
		spec,
		"sparse value of int(0) should be converted to false",
	)

	spec = lo.Must(bson.Marshal(bson.D{{sparseKey, float64(0)}}))
	spec, err = convertSparseSpecToBool(spec)
	s.Require().NoError(err)

	s.Assert().Equal(
		lo.Must(bson.Marshal(bson.D{{sparseKey, false}})),
		spec,
		"sparse value of float64(0) should be converted to false",
	)

	spec = lo.Must(bson.Marshal(bson.D{{sparseKey, float32(-1234)}}))
	spec, err = convertSparseSpecToBool(spec)
	s.Require().NoError(err)

	s.Assert().Equal(
		lo.Must(bson.Marshal(bson.D{{sparseKey, true}})),
		spec,
		"negative float32 sparse value should be converted to true",
	)

	spec = lo.Must(bson.Marshal(bson.D{{sparseKey, int(815)}}))
	spec, err = convertSparseSpecToBool(spec)
	s.Require().NoError(err)

	s.Assert().Equal(
		lo.Must(bson.Marshal(bson.D{{sparseKey, true}})),
		spec,
		"negative int(815) sparse value should be converted to true",
	)
}

func (s *UnitTestSuite) TestNormalizeSpec() {
	index := lo.Must(bson.Marshal(bson.D{
		{"v", 2},
		{"name", "indecks"},
		{sparseKey, 12345},
		{expireAfterSecondsKey, float32(12345)},
		{bitsKey, float32(12.2)},
	}))

	normalizedIndex := lo.Must(bson.Marshal(bson.D{
		{"v", 2},
		{"name", "indecks"},
		{sparseKey, true},
		{expireAfterSecondsKey, int32(12345)},
		{bitsKey, int32(12)},
	}))

	before := slices.Clone(index)
	index, err := normalizeTypesInSpec(index)
	s.Require().NoError(err)
	s.Assert().NotEqual(before, index, "should normalize spec")
	s.Require().Equal(normalizedIndex, index)

	// Since we pass in a normalized spec, it should not be modified.
	before = slices.Clone(normalizedIndex)
	normalizedIndex, err = normalizeTypesInSpec(normalizedIndex)
	s.Require().NoError(err)
	s.Assert().Equal(before, normalizedIndex, "should not normalize if already normalized")
}

func (s *UnitTestSuite) testIndexKeyConversions(
	key bson.D,
	expectedKey bson.D,
	expectModernization bool,
) {
	index, expectedIndex := getLegacyIndexAndConvertedIndex(key, expectedKey)

	index, didModernize, err := ModernizeSpec(index)
	s.Require().NoError(err)
	s.Assert().Equal(expectModernization, didModernize)

	s.Assert().Equal(expectedIndex, index, "expected %+v; got %+v", expectedIndex, index)
}
