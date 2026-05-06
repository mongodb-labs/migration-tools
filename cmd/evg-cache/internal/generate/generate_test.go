package generate_test

import (
	"bytes"
	"testing"

	"github.com/mongodb-labs/migration-tools/cmd/evg-cache/internal/cache"
	"github.com/mongodb-labs/migration-tools/cmd/evg-cache/internal/generate"
	"github.com/stretchr/testify/require"
)

func testConfig() cache.Config {
	return cache.NewBuilder("mise-and-go").
		WithBucket("mciuploads").
		WithNamespace("myproject/mise-cache").
		WithKeyFiles([]string{"mise.toml"}).
		WithCachePaths([]string{".local/bin/mise"}).
		WithScriptPrefix("./evg-cache-scripts").
		Build()
}

func TestGenerateFunctions_OutputIsYAML(t *testing.T) {
	var buf bytes.Buffer
	err := generate.GenerateFunctions(testConfig(), "mise tools", &buf)
	require.NoError(t, err, "GenerateFunctions should not return an error")
	output := buf.String()
	require.Contains(t, output, "functions:",
		"output should be a YAML functions block")
}

func TestGenerateFunctions_ContainsBothFunctionNames(t *testing.T) {
	var buf bytes.Buffer
	err := generate.GenerateFunctions(testConfig(), "mise tools", &buf)
	require.NoError(t, err)
	output := buf.String()
	require.Contains(t, output, "restore-mise-and-go-cache",
		"output should contain the restore function name")
	require.Contains(t, output, "save-mise-and-go-cache",
		"output should contain the save function name")
}

func TestGenerateFunctions_RestoreFunctionContainsS3Get(t *testing.T) {
	var buf bytes.Buffer
	err := generate.GenerateFunctions(testConfig(), "mise tools", &buf)
	require.NoError(t, err)
	output := buf.String()
	require.Contains(t, output, "s3.get",
		"restore function should include s3.get command")
	require.Contains(t, output, "optional: true",
		"s3.get in restore function should be optional so cache miss does not fail the task")
}

func TestGenerateFunctions_SaveFunctionContainsS3Put(t *testing.T) {
	var buf bytes.Buffer
	err := generate.GenerateFunctions(testConfig(), "mise tools", &buf)
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
	err := generate.GenerateFunctions(cfg, "mise tools", &buf)
	require.NoError(t, err)
	output := buf.String()
	require.Contains(t, output, cfg.S3Path(),
		"output should contain the full S3 path with namespace and expansion variables")
}

func TestGenerateFunctions_ScriptPrefixInOutput(t *testing.T) {
	var buf bytes.Buffer
	err := generate.GenerateFunctions(testConfig(), "mise tools", &buf)
	require.NoError(t, err)
	output := buf.String()
	require.Contains(t, output, "./evg-cache-scripts/set-distro-id-expansion.sh",
		"output should reference scripts via the configured script prefix")
}

func TestGenerateFunctions_DisplayNameInOutput(t *testing.T) {
	var buf bytes.Buffer
	err := generate.GenerateFunctions(testConfig(), "My Custom Display Name", &buf)
	require.NoError(t, err)
	output := buf.String()
	require.Contains(t, output, "My Custom Display Name",
		"s3.put display name should match the provided display name argument")
}
