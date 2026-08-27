package resumetoken

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestParse(t *testing.T) {
	cases := []struct {
		label    string
		data     string
		expected Parsed
	}{
		{
			label: "v2 event",
			data:  "826A8C8E62000000022B042C0100296E5A1004D802A00AB9E24901A92C71E85CC9ED82463C6F7065726174696F6E54797065003C696E736572740046646F63756D656E744B65790046645F696400646A8C8E62C1EC1FE8858052CF000004",
			expected: Parsed{
				Timestamp: bson.Timestamp{1787596386, 2},
				Version:   2,
				TokenType: TokenTypeEvent,
			},
		},
		{
			label: "v2 high-water mark",
			data:  "826A8C8E64000000012B0429296E1404",
			expected: Parsed{
				Timestamp: bson.Timestamp{1787596388, 1},
				Version:   2,
				TokenType: TokenTypeHighWaterMark,
			},
		},
		{
			label: "v1 event",
			data:  "826A8C8FBC000000022B022C0100296E5A1004F6E232028E324338BC654BED3745B94B46645F696400646A8C8FBCD1A0E0B8CD8052CF0004",
			expected: Parsed{
				Timestamp: bson.Timestamp{1787596732, 2},
				Version:   1,
				TokenType: TokenTypeEvent,
			},
		},
		{
			label: "v1 high-water mark",
			data:  "826A8C8FBF000000012B0229296E04",
			expected: Parsed{
				Timestamp: bson.Timestamp{1787596735, 1},
				Version:   1,
				TokenType: TokenTypeHighWaterMark,
			},
		},
	}

	for _, curCase := range cases {
		t.Run(curCase.label, func(t *testing.T) {
			rt, err := bson.Marshal(bson.D{{"_data", curCase.data}})
			require.NoError(t, err)

			parsed, err := Parse(rt)
			require.NoError(t, err)

			assert.Equal(t, curCase.expected, parsed)
		})
	}
}

func TestParse_Invalid(t *testing.T) {
	cases := []struct {
		label     string
		token     bson.D
		expectErr []string
	}{
		{
			label:     "no _data",
			token:     bson.D{{"foo", "bar"}},
			expectErr: []string{"_data"},
		},
		{
			label:     "_data is not a string",
			token:     bson.D{{"_data", 123}},
			expectErr: []string{"_data"},
		},
		{
			label:     "empty _data",
			token:     bson.D{{"_data", ""}},
			expectErr: []string{"key string type byte"},
		},
		{
			label:     "non-hex _data",
			token:     bson.D{{"_data", "zzzz"}},
			expectErr: []string{"key string type byte"},
		},
		{
			label:     "odd-length hex _data",
			token:     bson.D{{"_data", "826A8C8E6"}},
			expectErr: []string{"timestamp"},
		},
		{
			label:     "wrong key string type",
			token:     bson.D{{"_data", "836A8C8E64000000012B0429296E1404"}},
			expectErr: []string{"unexpected key string type", "131", "130"},
		},
		{
			label:     "truncated in timestamp",
			token:     bson.D{{"_data", "826A8C8E64"}},
			expectErr: []string{"read timestamp"},
		},
		{
			label:     "truncated after timestamp",
			token:     bson.D{{"_data", "826A8C8E64000000012B04"}},
			expectErr: []string{"after timestamp"},
		},
		{
			label:     "unknown version",
			token:     bson.D{{"_data", "826A8C8E64000000012B0629296E1404"}},
			expectErr: []string{"version bytes", "2b06"},
		},
		{
			label:     "unknown token type",
			token:     bson.D{{"_data", "826A8C8E64000000012B0499296E1404"}},
			expectErr: []string{"token type bytes", "99296e"},
		},
	}

	for _, curCase := range cases {
		t.Run(curCase.label, func(t *testing.T) {
			rt, err := bson.Marshal(curCase.token)
			require.NoError(t, err)

			parsed, err := Parse(rt)
			require.Error(t, err, "should fail to parse %v", curCase.token)
			assert.Equal(t, Parsed{}, parsed, "should return zero-value parse")

			for _, expect := range curCase.expectErr {
				assert.ErrorContains(t, err, expect)
			}
		})
	}
}
