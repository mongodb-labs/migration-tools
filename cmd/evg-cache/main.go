// cmd/evg-cache/main.go
package main

import (
	"fmt"
	"os"

	"github.com/mongodb-labs/migration-tools/cmd/evg-cache/internal/cache"
	"github.com/mongodb-labs/migration-tools/cmd/evg-cache/internal/extractscripts"
	"github.com/mongodb-labs/migration-tools/cmd/evg-cache/internal/generate"
	"github.com/mongodb-labs/migration-tools/cmd/evg-cache/internal/scripts"
	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:          "evg-cache",
		Short:        "Generate Evergreen CI YAML for S3-backed build caching",
		SilenceUsage: true,
	}
	root.AddCommand(generateCmd(), extractScriptsCmd())
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func generateCmd() *cobra.Command {
	var (
		name         string
		bucket       string
		namespace    string
		keyFiles     []string
		cachePaths   []string
		displayName  string
		scriptPrefix string
	)

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Print Evergreen functions YAML for a cache configuration",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if displayName == "" {
				displayName = name
			}
			cfg, buildErr := buildConfig(cacheParams{
				name:         name,
				bucket:       bucket,
				namespace:    namespace,
				keyFiles:     keyFiles,
				cachePaths:   cachePaths,
				scriptPrefix: scriptPrefix,
			})
			if buildErr != nil {
				return buildErr
			}
			return generate.GenerateFunctions(cfg, displayName, cmd.OutOrStdout())
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Cache name (required, matches [a-zA-Z0-9-]+)")
	cmd.Flags().StringVar(&bucket, "bucket", "", "S3 bucket name (required)")
	cmd.Flags().StringVar(&namespace, "namespace", "", "S3 namespace / path prefix (required)")
	cmd.Flags().
		StringArrayVar(&keyFiles, "key-file", nil, "File to hash for cache key (repeatable, required)")
	cmd.Flags().
		StringArrayVar(&cachePaths, "cache-path", nil, "Path to bundle into tarball (repeatable, required)")
	cmd.Flags().
		StringVar(&displayName, "display-name", "", "Human-readable name for S3 put (defaults to --name)")
	cmd.Flags().
		StringVar(&scriptPrefix, "script-prefix", "./evg-cache-scripts", "Path prefix where runtime scripts live on the agent")

	mustRequire(cmd, "name")
	mustRequire(cmd, "bucket")
	mustRequire(cmd, "namespace")
	mustRequire(cmd, "key-file")
	mustRequire(cmd, "cache-path")

	return cmd
}

func extractScriptsCmd() *cobra.Command {
	var outputDir string

	cmd := &cobra.Command{
		Use:   "extract-scripts",
		Short: "Write runtime scripts to disk for use in Evergreen tasks",
		RunE: func(_ *cobra.Command, _ []string) error {
			if err := extractscripts.ExtractScripts(scripts.FS, outputDir); err != nil {
				return fmt.Errorf("extracting scripts to %q: %w", outputDir, err)
			}
			fmt.Fprintf(os.Stderr, "Scripts written to %s\n", outputDir)
			return nil
		},
	}

	cmd.Flags().
		StringVar(&outputDir, "output-dir", "", "Directory to write scripts into (required)")
	mustRequire(cmd, "output-dir")

	return cmd
}

type cacheParams struct {
	name         string
	bucket       string
	namespace    string
	keyFiles     []string
	cachePaths   []string
	scriptPrefix string
}

func buildConfig(p cacheParams) (cfg cache.Config, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("invalid cache configuration: %v", r)
		}
	}()
	cfg = cache.NewBuilder(p.name).
		WithBucket(p.bucket).
		WithNamespace(p.namespace).
		WithKeyFiles(p.keyFiles).
		WithCachePaths(p.cachePaths).
		WithScriptPrefix(p.scriptPrefix).
		Build()
	return cfg, nil
}

func mustRequire(cmd *cobra.Command, flagName string) {
	if err := cmd.MarkFlagRequired(flagName); err != nil {
		panic(fmt.Sprintf("failed to mark flag %q as required: %s", flagName, err))
	}
}
