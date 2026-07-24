package mongotools

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMaybeAddDirectConnection(t *testing.T) {
	cases := [][2]string{
		{
			"mongodb://johndoe:password@example.com/?authSource=admin&ssl=true",
			"mongodb://johndoe:password@example.com/?authSource=admin&ssl=true&directConnection=true",
		},
		{
			"mongodb://foo:bar@ip-10-0-0-208.ec2.internal:27019/?authSource=admin",
			"mongodb://foo:bar@ip-10-0-0-208.ec2.internal:27019/?authSource=admin&directConnection=true",
		},
	}

	for _, cur := range cases {
		changed, got, err := MaybeAddDirectConnection(cur[0])
		require.NoError(t, err, "in: %#q", cur[0])

		assert.Equal(t, cur[0] != cur[1], changed, "changed-ness")
		assert.Equal(t, cur[1], got, "check expected")
	}
}
