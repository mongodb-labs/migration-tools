//go:build unix

package fstools

import (
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"syscall"
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

	// The relative target resolves against the link’s own directory.
	linkPath := filepath.Join(dir, "link")
	require.NoError(t, os.Symlink("file", linkPath))

	fifoPath := filepath.Join(dir, "fifo")
	require.NoError(t, syscall.Mkfifo(fifoPath, 0o644))

	// Unix socket paths have a short length limit (104 bytes on macOS),
	// which t.TempDir() paths can exceed, so put the socket under /tmp.
	//
	//nolint:usetesting // t.TempDir() doesn’t guarantee a path under /tmp.
	sockDir, err := os.MkdirTemp("/tmp", "fstools-test-")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(sockDir) })

	sockPath := filepath.Join(sockDir, "sock")
	listener, err := net.Listen("unix", sockPath)
	require.NoError(t, err)
	defer listener.Close()

	tests := []struct {
		path string
		want string
	}{
		{dir, "directory"},
		{filePath, "regular file"},
		{linkPath, "symbolic link"},
		{fifoPath, "named pipe"},
		{sockPath, "socket"},
		{"/dev/null", "character device"},
	}

	if blockDevice := findBlockDevice(t); blockDevice != "" {
		tests = append(tests, struct {
			path string
			want string
		}{blockDevice, "device"})
	} else {
		t.Log("No block device found under /dev; skipping that case.")
	}

	for _, tt := range tests {
		info, err := os.Lstat(tt.path)
		require.NoError(t, err, "lstat %#q", tt.path)

		assert.Equal(t, tt.want, DescribeFileType(info.Mode()), "path %#q", tt.path)
	}
}

// findBlockDevice returns the path of some block device under /dev, or
// empty string if there is none (e.g., in some containers).
func findBlockDevice(t *testing.T) string {
	t.Helper()

	entries, err := os.ReadDir("/dev")
	require.NoError(t, err, "read /dev")

	for _, entry := range entries {
		mode := entry.Type()
		if mode&fs.ModeDevice != 0 && mode&fs.ModeCharDevice == 0 {
			return filepath.Join("/dev", entry.Name())
		}
	}

	return ""
}
