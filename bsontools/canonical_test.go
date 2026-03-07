package bsontools

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestSortFields(t *testing.T) {
	cases := []struct {
		name   string
		in     bson.D
		expect bson.D
	}{
		{
			name:   "empty document",
			in:     bson.D{},
			expect: bson.D{},
		},
		{
			name:   "single field",
			in:     bson.D{{"foo", 1}},
			expect: bson.D{{"foo", 1}},
		},
		{
			name:   "simple sort",
			in:     bson.D{{"foo", 1}, {"bar", 1}},
			expect: bson.D{{"bar", 1}, {"foo", 1}},
		},
		{
			name:   "check sort stability",
			in:     bson.D{{"bar", 2}, {"foo", 1}, {"bar", 1}},
			expect: bson.D{{"bar", 2}, {"bar", 1}, {"foo", 1}},
		},
		{
			name: "deeply nested documents",
			in: bson.D{
				{"bbb", bson.D{
					{"foo", 1},
					{"bar", bson.D{
						{"z", 1},
						{"a", 2},
					}},
				}},
				{"aaa", 1},
			},
			expect: bson.D{
				{"aaa", 1},
				{"bbb", bson.D{
					{"bar", bson.D{
						{"a", 2},
						{"z", 1},
					}},
					{"foo", 1},
				}},
			},
		},
		{
			name: "array of documents (array keys should NOT sort, contents SHOULD)",
			in: bson.D{
				{"z", 1},
				{"arr", bson.A{
					bson.D{{"foo", 2}, {"bar", 1}}, // index "0"
					bson.D{{"c", 3}, {"a", 4}},     // index "1"
				}},
				{"a", 2},
			},
			expect: bson.D{
				{"a", 2},
				{"arr", bson.A{
					bson.D{{"bar", 1}, {"foo", 2}},
					bson.D{{"a", 4}, {"c", 3}},
				}},
				{"z", 1},
			},
		},
		{
			name: "mixed types and variable lengths (stresses the scratch buffer)",
			in: bson.D{
				{"z", "a very long string that takes up lots of bytes"},
				{"y", true},
				{"x", int64(1234567890)},
				{"w", nil},
				{"v", bson.D{{"b", 1}, {"a", 2}}},
			},
			expect: bson.D{
				{"v", bson.D{{"a", 2}, {"b", 1}}},
				{"w", nil},
				{"x", int64(1234567890)},
				{"y", true},
				{"z", "a very long string that takes up lots of bytes"},
			},
		},
		{
			name: "empty embedded docs and arrays",
			in: bson.D{
				{"z", bson.D{}},
				{"y", bson.A{}},
				{"x", 1},
			},
			expect: bson.D{
				{"x", 1},
				{"y", bson.A{}},
				{"z", bson.D{}},
			},
		},
	}

	for _, curCase := range cases {
		t.Run(curCase.name, func(t *testing.T) {
			in, err := bson.Marshal(curCase.in)
			require.NoError(t, err)

			expect, err := bson.Marshal(curCase.expect)
			require.NoError(t, err)

			got := slices.Clone(bson.Raw(in))
			err = SortFields(got)
			require.NoError(t, err)

			assert.Equal(t, bson.Raw(expect), got, "expect %v; got %v", bson.Raw(expect), got)
		})
	}
}
