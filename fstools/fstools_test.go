package fstools

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDescribeFileType(t *testing.T) {
	tests := []struct {
		mode fs.FileMode
		want string
	}{
		{0, "regular file"},
		{0o644, "regular file"},
		{fs.ModeDir, "directory"},
		{fs.ModeDir | 0o755, "directory"},
		{fs.ModeSymlink, "symbolic link"},
		{fs.ModeSymlink | 0o777, "symbolic link"},
		{fs.ModeNamedPipe, "named pipe"},
		{fs.ModeSocket, "socket"},
		{fs.ModeDevice, "device"},

		// Character devices also have ModeDevice set.
		{fs.ModeDevice | fs.ModeCharDevice, "character device"},
		{fs.ModeCharDevice, "character device"},

		// Non-type bits don’t affect the result.
		{0o644 | fs.ModeSetuid | fs.ModeSticky, "regular file"},
		{fs.ModeDir | fs.ModeSticky | 0o1777, "directory"},

		// Unrecognized types fall through to the default.
		{fs.ModeIrregular, "type ?---------"},
		{fs.ModeIrregular | 0o644, "type ?---------"},
	}

	for _, tt := range tests {
		assert.Equal(
			t,
			tt.want,
			DescribeFileType(tt.mode),
			"mode %s (%#o)",
			tt.mode,
			uint32(tt.mode),
		)
	}
}

func TestDescribeFileType_RealFiles(t *testing.T) {
	dir := t.TempDir()

	filePath := filepath.Join(dir, "file")
	require.NoError(t, os.WriteFile(filePath, []byte("hello"), 0o644))

	linkPath := filepath.Join(dir, "link")
	require.NoError(t, os.Symlink(filePath, linkPath))

	tests := []struct {
		path string
		want string
	}{
		{dir, "directory"},
		{filePath, "regular file"},
		{linkPath, "symbolic link"},
	}

	for _, tt := range tests {
		info, err := os.Lstat(tt.path)
		require.NoError(t, err, "lstat %#q", tt.path)

		assert.Equal(t, tt.want, DescribeFileType(info.Mode()), "path %#q", tt.path)
	}
}
