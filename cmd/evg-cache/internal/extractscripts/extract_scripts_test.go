package extractscripts_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mongodb-labs/migration-tools/cmd/evg-cache/internal/extractscripts"
	"github.com/mongodb-labs/migration-tools/cmd/evg-cache/internal/scripts"
	"github.com/stretchr/testify/require"
)

func TestExtractScripts_WritesAllScripts(t *testing.T) {
	outDir := t.TempDir()
	err := extractscripts.ExtractScripts(scripts.FS, outDir)
	require.NoError(t, err, "ExtractScripts should not return an error")

	expectedFiles := []string{
		"set-distro-id-expansion.sh",
		"run-python-script.sh",
		"find-recent-python.sh",
		"compute-cache-key.py",
		"restore-cache-artifact.py",
		"create-cache-artifact.py",
	}
	for _, name := range expectedFiles {
		path := filepath.Join(outDir, name)
		_, statErr := os.Stat(path)
		require.NoError(t, statErr, "expected script file to exist: %s", name)
	}
}

func TestExtractScripts_ShellScriptsAreExecutable(t *testing.T) {
	outDir := t.TempDir()
	err := extractscripts.ExtractScripts(scripts.FS, outDir)
	require.NoError(t, err, "ExtractScripts should not return an error")

	shellScriptFound := false
	walkErr := fs.WalkDir(scripts.FS, "data", func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".sh") {
			return err
		}
		shellScriptFound = true
		info, statErr := os.Stat(filepath.Join(outDir, d.Name()))
		require.NoError(t, statErr)
		mode := info.Mode()
		require.NotEqual(t, fs.FileMode(0), mode&0o111,
			"shell script should be executable: %s (mode: %v)", d.Name(), mode)
		return nil
	})
	require.NoError(t, walkErr)
	require.True(t, shellScriptFound, "at least one shell script should exist in the embedded FS")
}

func TestExtractScripts_FileContentsMatch(t *testing.T) {
	outDir := t.TempDir()
	err := extractscripts.ExtractScripts(scripts.FS, outDir)
	require.NoError(t, err, "ExtractScripts should not return an error")

	walkErr := fs.WalkDir(scripts.FS, "data", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		embedded, readErr := fs.ReadFile(scripts.FS, path)
		require.NoError(t, readErr, "should be able to read embedded file %s", d.Name())

		extracted, readErr := os.ReadFile(filepath.Join(outDir, d.Name()))
		require.NoError(t, readErr, "should be able to read extracted file %s", d.Name())

		require.Equal(t, embedded, extracted,
			"extracted %s should have identical contents to the embedded original", d.Name())
		return nil
	})
	require.NoError(t, walkErr)
}

func TestExtractScripts_CreatesOutputDirIfMissing(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "nested", "new-dir")
	err := extractscripts.ExtractScripts(scripts.FS, outDir)
	require.NoError(t, err, "ExtractScripts should create missing output directories")

	entries, readErr := os.ReadDir(outDir)
	require.NoError(t, readErr)
	require.NotEmpty(t, entries, "output directory should contain files after extraction")
}

func TestExtractScripts_PythonScriptsUseCorrectPermissions(t *testing.T) {
	outDir := t.TempDir()
	err := extractscripts.ExtractScripts(scripts.FS, outDir)
	require.NoError(t, err, "ExtractScripts should not return an error")

	pythonScripts := []string{
		"compute-cache-key.py",
		"restore-cache-artifact.py",
		"create-cache-artifact.py",
	}
	for _, name := range pythonScripts {
		path := filepath.Join(outDir, name)
		info, statErr := os.Stat(path)
		require.NoError(t, statErr)
		require.Equal(t, fs.FileMode(0o644), info.Mode().Perm(),
			"python script should have 0644 permissions (not executable): %s", name)
	}
}
