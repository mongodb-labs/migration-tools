package generate_test

import (
	"bytes"
	"strings"
	"testing"

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
		Build()
}

func TestGenerateFunctions_OutputIsYAML(t *testing.T) {
	var buf bytes.Buffer
	err := generate.GenerateFunctions(testConfig(), "", false, &buf)
	require.NoError(t, err, "GenerateFunctions should not return an error")
	output := buf.String()
	require.Contains(t, output, "functions:",
		"output should be a YAML functions block")
}

func TestGenerateFunctions_ContainsBothFunctionNames(t *testing.T) {
	var buf bytes.Buffer
	err := generate.GenerateFunctions(testConfig(), "", false, &buf)
	require.NoError(t, err)
	output := buf.String()
	require.Contains(t, output, "restore-mise-and-go-cache",
		"output should contain the restore function name")
	require.Contains(t, output, "save-mise-and-go-cache",
		"output should contain the save function name")
}

func TestGenerateFunctions_ContainsSetupFunctionName(t *testing.T) {
	var buf bytes.Buffer
	err := generate.GenerateFunctions(testConfig(), "", false, &buf)
	require.NoError(t, err)
	output := buf.String()
	// Setup function name is always "set-up-evg-cache-scripts" regardless of the
	// cache name, so multiple caches sharing a root produce one setup function.
	require.Contains(t, output, "set-up-evg-cache-scripts",
		"setup function name should appear in the YAML output")
}

func TestGenerateFunctions_SetupFunctionName(t *testing.T) {
	require.Equal(t, "set-up-evg-cache-scripts", generate.SetupFunctionName,
		"SetupFunctionName constant should be 'set-up-evg-cache-scripts'")
}

func TestGenerateFunctions_TwoCachesSameRootProduceSameSetupName(t *testing.T) {
	b1, err := cache.NewBuilder("mise-and-go")
	require.NoError(t, err)
	cfg1 := b1.WithBucket("b").WithNamespace("ns").
		WithKeyFiles([]string{"f"}).WithCachePaths([]string{"p"}).
		Build()

	b2, err := cache.NewBuilder("go-modules")
	require.NoError(t, err)
	cfg2 := b2.WithBucket("b").WithNamespace("ns").
		WithKeyFiles([]string{"f"}).WithCachePaths([]string{"p"}).
		Build()

	var buf1, buf2 bytes.Buffer
	require.NoError(t, generate.GenerateFunctions(cfg1, "", false, &buf1))
	require.NoError(t, generate.GenerateFunctions(cfg2, "", false, &buf2))

	require.Contains(t, buf1.String(), "set-up-evg-cache-scripts",
		"first cache should use the shared setup function name")
	require.Contains(t, buf2.String(), "set-up-evg-cache-scripts",
		"second cache should use the same shared setup function name")
}

func TestGenerateFunctions_OmitSetupFunctionExcludesSetupFromOutput(t *testing.T) {
	var buf bytes.Buffer
	err := generate.GenerateFunctions(testConfig(), "", true, &buf)
	require.NoError(t, err, "GenerateFunctions with omitSetup=true should not return an error")
	output := buf.String()
	require.NotContains(t, output, "set-up-evg-cache-scripts",
		"setup function should be absent from output when omitSetup is true")
	require.Contains(t, output, "restore-mise-and-go-cache",
		"restore function should still be present when omitSetup is true")
	require.Contains(t, output, "save-mise-and-go-cache",
		"save function should still be present when omitSetup is true")
}

func TestGenerateFunctions_IncludeSetupFunctionIncludesSetupInOutput(t *testing.T) {
	var buf bytes.Buffer
	err := generate.GenerateFunctions(testConfig(), "", false, &buf)
	require.NoError(t, err, "GenerateFunctions with omitSetup=false should not return an error")
	output := buf.String()
	require.Contains(t, output, "set-up-evg-cache-scripts",
		"setup function should be present in output when omitSetup is false")
}

func TestGenerateFunctions_RestoreFunctionContainsS3Get(t *testing.T) {
	var buf bytes.Buffer
	err := generate.GenerateFunctions(testConfig(), "", false, &buf)
	require.NoError(t, err)
	output := buf.String()
	require.Contains(t, output, "s3.get",
		"restore function should include s3.get command")
	require.Contains(t, output, "optional: true",
		"s3.get in restore function should be optional so cache miss does not fail the task")
}

func TestGenerateFunctions_SaveFunctionContainsS3Put(t *testing.T) {
	var buf bytes.Buffer
	err := generate.GenerateFunctions(testConfig(), "", false, &buf)
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
	err := generate.GenerateFunctions(cfg, "", false, &buf)
	require.NoError(t, err)
	output := buf.String()
	require.Contains(t, output, cfg.S3Path(),
		"output should contain the full S3 path with namespace and expansion variables")
}

func TestGenerateFunctions_ScriptDirInOutput(t *testing.T) {
	var buf bytes.Buffer
	err := generate.GenerateFunctions(testConfig(), "", false, &buf)
	require.NoError(t, err)
	output := buf.String()
	require.Contains(t, output, "./evg-cache-scripts/set-distro-id-expansion.sh",
		"output should reference scripts via the configured script dir")
}

func TestGenerateFunctions_DisplayNameInOutput(t *testing.T) {
	b, err := cache.NewBuilder("mise-and-go")
	require.NoError(t, err)
	cfg := b.
		WithBucket("mciuploads").
		WithNamespace("myproject/mise-cache").
		WithKeyFiles([]string{"mise.toml"}).
		WithCachePaths([]string{".local/bin/mise"}).
		WithDisplayName("My Custom Display Name").
		Build()

	var buf bytes.Buffer
	require.NoError(t, generate.GenerateFunctions(cfg, "", false, &buf))
	output := buf.String()
	require.Contains(t, output, "My Custom Display Name",
		"s3.put display_name should match the display name set on the cache config")
}

func TestGenerateFunctions_SetupFunctionIsIdempotent(t *testing.T) {
	var buf bytes.Buffer
	err := generate.GenerateFunctions(testConfig(), "", false, &buf)
	require.NoError(t, err)
	output := buf.String()
	// The setup script must exit early if the sentinel file is already present
	// so that multiple included YAML files defining the same function are harmless.
	require.Contains(t, output, ".setup-complete",
		"setup function should check for a sentinel file to skip redundant writes")
}

func TestGenerateFunctions_SetupFunctionWritesScripts(t *testing.T) {
	var buf bytes.Buffer
	err := generate.GenerateFunctions(testConfig(), "", false, &buf)
	require.NoError(t, err)
	output := buf.String()
	require.Contains(t, output, "compute-cache-key.py",
		"setup function should reference the Python script filename")
	require.Contains(t, output, "run-python-script.sh",
		"setup function should reference the shell script filename")
}

func TestGenerateFunctions_SetupFunctionMakesShellScriptsExecutable(t *testing.T) {
	var buf bytes.Buffer
	err := generate.GenerateFunctions(testConfig(), "", false, &buf)
	require.NoError(t, err)
	output := buf.String()
	// With UseLiteralStyleIfMultiline the script is a YAML literal block, so
	// double quotes appear verbatim (no backslash escaping).
	require.Contains(t, output, `chmod +x "./evg-cache-scripts/run-python-script.sh"`,
		"setup function should make shell scripts executable")
	require.NotContains(t, output, `chmod +x "./evg-cache-scripts/compute-cache-key.py"`,
		"setup function should not make Python scripts executable")
}

func TestGenerateFunctions_HeaderComment(t *testing.T) {
	var buf bytes.Buffer
	header := "This file was generated by running `evg-cache generate --name foo`. Do not edit this file directly!"
	err := generate.GenerateFunctions(testConfig(), header, false, &buf)
	require.NoError(t, err, "GenerateFunctions with a header should not return an error")
	output := buf.String()
	require.Contains(t, output, "# "+header,
		"output should include the complete header text as a YAML comment")
	require.True(t, len(output) > 0 && output[0] == '#',
		"header comment should appear at the very start of the output")
}

func TestGenerateFunctions_NoHeaderComment(t *testing.T) {
	var buf bytes.Buffer
	err := generate.GenerateFunctions(testConfig(), "", false, &buf)
	require.NoError(t, err, "GenerateFunctions with empty header should not return an error")
	output := buf.String()
	// Without a header the output should start directly with the YAML content, not a comment line.
	require.True(t, strings.HasPrefix(output, "functions:"),
		"output should start with 'functions:' when no header is provided")
}
