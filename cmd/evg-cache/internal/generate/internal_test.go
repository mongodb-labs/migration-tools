package generate

import (
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
)

func TestSetupScriptsCommands_ErrorOnDuplicateBasename(t *testing.T) {
	dupFS := fstest.MapFS{
		"sub/foo.sh":   {Data: []byte("#!/bin/bash\n")},
		"other/foo.sh": {Data: []byte("#!/bin/bash\n")},
	}
	_, err := setupScriptsCommands(dupFS, "./scripts")
	require.Error(t, err, "duplicate basenames in the scripts FS should produce an error")
	require.Contains(t, err.Error(), "foo.sh",
		"error should name the colliding basename")
}
