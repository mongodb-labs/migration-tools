package generate_test

import (
	"bytes"
	"testing"
	"testing/fstest"

	"github.com/mongodb-labs/migration-tools/cmd/evg-cache/internal/cache"
	"github.com/mongodb-labs/migration-tools/cmd/evg-cache/internal/generate"
	"github.com/stretchr/testify/require"
)

func testConfig() cache.Config {
	b, err := cache.NewBuilder("mise-and-go")
	if err != nil {
		panic(err) // "mise-and-go" is always a valid name
	}
	return b.
		WithBucket("mciuploads").
		WithNamespace("myproject/mise-cache").
		WithKeyFiles([]string{"mise.toml"}).
		WithCachePaths([]string{".local/bin/mise"}).
		WithScriptPrefix("./evg-cache-scripts").
		Build()
}

// testScriptsFS returns a minimal fs.FS with one shell script and one Python
// script so tests can verify setup-function generation without depending on
// the real embedded scripts.
func testScriptsFS() fstest.MapFS {
	return fstest.MapFS{
		"run.sh":    {Data: []byte("#!/bin/bash\necho hi\n")},
		"helper.py": {Data: []byte("#!/usr/bin/env python3\nprint('hello')\n")},
	}
}

func TestGenerateFunctions_OutputIsYAML(t *testing.T) {
	var buf bytes.Buffer
	err := generate.GenerateFunctions(testConfig(), "mise tools", testScriptsFS(), true, &buf)
	require.NoError(t, err, "GenerateFunctions should not return an error")
	output := buf.String()
	require.Contains(t, output, "functions:",
		"output should be a YAML functions block")
}

func TestGenerateFunctions_ContainsBothFunctionNames(t *testing.T) {
	var buf bytes.Buffer
	err := generate.GenerateFunctions(testConfig(), "mise tools", testScriptsFS(), true, &buf)
	require.NoError(t, err)
	output := buf.String()
	require.Contains(t, output, "restore-mise-and-go-cache",
		"output should contain the restore function name")
	require.Contains(t, output, "save-mise-and-go-cache",
		"output should contain the save function name")
}

func TestGenerateFunctions_ContainsSetupFunctionName(t *testing.T) {
	var buf bytes.Buffer
	err := generate.GenerateFunctions(testConfig(), "mise tools", testScriptsFS(), true, &buf)
	require.NoError(t, err)
	output := buf.String()
	// Setup function name is derived from the script prefix directory, not the
	// cache name, so multiple caches sharing a prefix produce one setup function.
	require.Contains(t, output, "setup-evg-cache-scripts",
		"setup function name should be derived from the script prefix directory")
}

func TestGenerateFunctions_SetupFunctionName(t *testing.T) {
	cfg := testConfig()
	require.Equal(t, "setup-evg-cache-scripts", generate.SetupFunctionName(cfg),
		"SetupFunctionName should return 'setup-' + base of ScriptPrefix")
}

func TestGenerateFunctions_IncludeSetupFalse_OmitsSetupFunction(t *testing.T) {
	var buf bytes.Buffer
	err := generate.GenerateFunctions(testConfig(), "mise tools", testScriptsFS(), false, &buf)
	require.NoError(t, err)
	output := buf.String()
	require.NotContains(t, output, "setup-evg-cache-scripts",
		"setup function should be omitted when includeSetup is false")
	require.Contains(t, output, "restore-mise-and-go-cache",
		"restore function should still be present when setup is omitted")
	require.Contains(t, output, "save-mise-and-go-cache",
		"save function should still be present when setup is omitted")
}

func TestGenerateFunctions_TwoCachesSamePrefixProduceSameSetupName(t *testing.T) {
	b1, err := cache.NewBuilder("mise-and-go")
	require.NoError(t, err)
	cfg1 := b1.WithBucket("b").WithNamespace("ns").
		WithKeyFiles([]string{"f"}).WithCachePaths([]string{"p"}).
		WithScriptPrefix("./evg-cache-scripts").Build()

	b2, err := cache.NewBuilder("go-modules")
	require.NoError(t, err)
	cfg2 := b2.WithBucket("b").WithNamespace("ns").
		WithKeyFiles([]string{"f"}).WithCachePaths([]string{"p"}).
		WithScriptPrefix("./evg-cache-scripts").Build()

	var buf1, buf2 bytes.Buffer
	require.NoError(t, generate.GenerateFunctions(cfg1, "n1", testScriptsFS(), true, &buf1))
	require.NoError(t, generate.GenerateFunctions(cfg2, "n2", testScriptsFS(), true, &buf2))

	require.Contains(t, buf1.String(), "setup-evg-cache-scripts",
		"first cache should use the shared setup function name")
	require.Contains(t, buf2.String(), "setup-evg-cache-scripts",
		"second cache should use the same shared setup function name")
}

func TestGenerateFunctions_RestoreFunctionContainsS3Get(t *testing.T) {
	var buf bytes.Buffer
	err := generate.GenerateFunctions(testConfig(), "mise tools", testScriptsFS(), true, &buf)
	require.NoError(t, err)
	output := buf.String()
	require.Contains(t, output, "s3.get",
		"restore function should include s3.get command")
	require.Contains(t, output, "optional: true",
		"s3.get in restore function should be optional so cache miss does not fail the task")
}

func TestGenerateFunctions_SaveFunctionContainsS3Put(t *testing.T) {
	var buf bytes.Buffer
	err := generate.GenerateFunctions(testConfig(), "mise tools", testScriptsFS(), true, &buf)
	require.NoError(t, err)
	output := buf.String()
	require.Contains(t, output, "s3.put",
		"save function should include s3.put command")
	require.Contains(t, output, "skip_existing",
		"s3.put should skip existing objects to avoid overwriting a populated cache")
}

func TestGenerateFunctions_ContainsS3Path(t *testing.T) {
	cfg := testConfig()
	var buf bytes.Buffer
	err := generate.GenerateFunctions(cfg, "mise tools", testScriptsFS(), true, &buf)
	require.NoError(t, err)
	output := buf.String()
	require.Contains(t, output, cfg.S3Path(),
		"output should contain the full S3 path with namespace and expansion variables")
}

func TestGenerateFunctions_ScriptPrefixInOutput(t *testing.T) {
	var buf bytes.Buffer
	err := generate.GenerateFunctions(testConfig(), "mise tools", testScriptsFS(), true, &buf)
	require.NoError(t, err)
	output := buf.String()
	require.Contains(t, output, "./evg-cache-scripts/set-distro-id-expansion.sh",
		"output should reference scripts via the configured script prefix")
}

func TestGenerateFunctions_DisplayNameInOutput(t *testing.T) {
	var buf bytes.Buffer
	err := generate.GenerateFunctions(
		testConfig(),
		"My Custom Display Name",
		testScriptsFS(),
		true,
		&buf,
	)

	require.NoError(t, err)
	output := buf.String()
	require.Contains(t, output, "My Custom Display Name",
		"s3.put display name should match the provided display name argument")
}

func TestGenerateFunctions_SetupFunctionIsIdempotent(t *testing.T) {
	var buf bytes.Buffer
	err := generate.GenerateFunctions(testConfig(), "mise tools", testScriptsFS(), true, &buf)
	require.NoError(t, err)
	output := buf.String()
	// The setup script must exit early if the sentinel file is already present
	// so that multiple included YAML files defining the same function are harmless.
	require.Contains(t, output, ".setup-complete",
		"setup function should check for a sentinel file to skip redundant writes")
}

func TestGenerateFunctions_SetupFunctionWritesScripts(t *testing.T) {
	var buf bytes.Buffer
	err := generate.GenerateFunctions(testConfig(), "mise tools", testScriptsFS(), true, &buf)
	require.NoError(t, err)
	output := buf.String()
	require.Contains(t, output, "run.sh",
		"setup function should reference the shell script filename")
	require.Contains(t, output, "helper.py",
		"setup function should reference the Python script filename")
}

func TestGenerateFunctions_SetupFunctionMakesShellScriptsExecutable(t *testing.T) {
	var buf bytes.Buffer
	err := generate.GenerateFunctions(testConfig(), "mise tools", testScriptsFS(), true, &buf)
	require.NoError(t, err)
	output := buf.String()
	// With UseLiteralStyleIfMultiline the script is a YAML literal block, so
	// double quotes appear verbatim (no backslash escaping).
	require.Contains(t, output, `chmod +x "./evg-cache-scripts/run.sh"`,
		"setup function should make shell scripts executable")
	require.NotContains(t, output, `chmod +x "./evg-cache-scripts/helper.py"`,
		"setup function should not make Python scripts executable")
}
