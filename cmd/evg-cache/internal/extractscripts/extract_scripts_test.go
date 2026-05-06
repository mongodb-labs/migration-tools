package extractscripts_test

import (
	"io/fs"
	"os"
	"path/filepath"
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
		"detect-cache-hit.py",
		"extract-cache-artifact.py",
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

	shellScripts := []string{
		"set-distro-id-expansion.sh",
		"run-python-script.sh",
		"find-recent-python.sh",
	}
	for _, name := range shellScripts {
		path := filepath.Join(outDir, name)
		info, statErr := os.Stat(path)
		require.NoError(t, statErr)
		mode := info.Mode()
		require.NotEqual(t, fs.FileMode(0), mode&0o111,
			"shell script should be executable: %s (mode: %v)", name, mode)
	}
}

func TestExtractScripts_FileContentsMatch(t *testing.T) {
	outDir := t.TempDir()
	err := extractscripts.ExtractScripts(scripts.FS, outDir)
	require.NoError(t, err, "ExtractScripts should not return an error")

	// Spot-check that compute-cache-key.py contains expected content.
	content, readErr := os.ReadFile(filepath.Join(outDir, "compute-cache-key.py"))
	require.NoError(t, readErr)
	require.Contains(t, string(content), "hashlib.sha256",
		"compute-cache-key.py should contain SHA256 hashing logic")
}

func TestExtractScripts_CreatesOutputDirIfMissing(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "nested", "new-dir")
	err := extractscripts.ExtractScripts(scripts.FS, outDir)
	require.NoError(t, err, "ExtractScripts should create missing output directories")

	entries, readErr := os.ReadDir(outDir)
	require.NoError(t, readErr)
	require.NotEmpty(t, entries, "output directory should contain files after extraction")
}

func TestExtractScripts_RunPythonScriptSourcesCorrectPath(t *testing.T) {
	outDir := t.TempDir()
	err := extractscripts.ExtractScripts(scripts.FS, outDir)
	require.NoError(t, err, "ExtractScripts should not return an error")

	content, readErr := os.ReadFile(filepath.Join(outDir, "run-python-script.sh"))
	require.NoError(t, readErr)
	require.Contains(t, string(content), `"$SCRIPT_DIR/find-recent-python.sh"`,
		"run-python-script.sh should source find-recent-python.sh from its own directory")
	require.NotContains(t, string(content), "../../etc",
		"run-python-script.sh should not reference mongosync-specific relative paths")
}

func TestExtractScripts_PythonScriptsUseCorrectPermissions(t *testing.T) {
	outDir := t.TempDir()
	err := extractscripts.ExtractScripts(scripts.FS, outDir)
	require.NoError(t, err, "ExtractScripts should not return an error")

	pythonScripts := []string{
		"compute-cache-key.py",
		"detect-cache-hit.py",
		"extract-cache-artifact.py",
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
