package bsontools

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestSortFields(t *testing.T) {
	cases := []struct {
		in, expect bson.D
	}{
		{
			in:     bson.D{},
			expect: bson.D{},
		},
		{
			in:     bson.D{{"foo", 1}},
			expect: bson.D{{"foo", 1}},
		},

		// simple:
		{
			in:     bson.D{{"foo", 1}, {"bar", 1}},
			expect: bson.D{{"bar", 1}, {"foo", 1}},
		},

		// check sort stability:
		{
			in:     bson.D{{"bar", 2}, {"foo", 1}, {"bar", 1}},
			expect: bson.D{{"bar", 2}, {"bar", 1}, {"foo", 1}},
		},

		// deep:
		{
			in: bson.D{
				{"bbb", bson.D{
					{"foo", 1},
					{"bar", -2},
				}},
				{"aaa", 1},
			},
			expect: bson.D{
				{"aaa", 1},
				{"bbb", bson.D{
					{"bar", -2},
					{"foo", 1},
				}},
			},
		},
	}

	for _, curCase := range cases {
		in, err := bson.Marshal(curCase.in)
		require.NoError(t, err)

		expect, err := bson.Marshal(curCase.expect)
		require.NoError(t, err)

		got, err := SortFields(in)
		require.NoError(t, err)

		assert.Equal(t, bson.Raw(expect), got, "expect %v; got %v", bson.Raw(expect), got)
	}
}
