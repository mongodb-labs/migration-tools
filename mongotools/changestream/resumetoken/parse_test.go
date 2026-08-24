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
