// cmd/evg-cache/main.go
package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/goccy/go-yaml"
	"github.com/mongodb-labs/migration-tools/cmd/evg-cache/internal/cache"
	"github.com/mongodb-labs/migration-tools/cmd/evg-cache/internal/extractscripts"
	"github.com/mongodb-labs/migration-tools/cmd/evg-cache/internal/generate"
	"github.com/mongodb-labs/migration-tools/cmd/evg-cache/internal/scripts"
	"github.com/urfave/cli/v3"
)

func main() {
	app := &cli.Command{
		Name:  "evg-cache",
		Usage: "Generate Evergreen CI YAML for S3-backed build caching",
		Commands: []*cli.Command{
			generateCmd(),
			extractScriptsCmd(),
		},
	}
	if err := app.Run(context.Background(), os.Args); err != nil {
		os.Exit(1)
	}
}

func generateCmd() *cli.Command {
	return &cli.Command{
		Name:  "generate",
		Usage: "Write Evergreen functions YAML for a cache configuration",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "name",
				Required: true,
				Usage:    "Cache name (required, matches [a-zA-Z0-9-]+)",
			},
			&cli.StringFlag{
				Name:     "bucket",
				Required: true,
				Usage:    "S3 bucket name",
			},
			&cli.StringFlag{
				Name:     "namespace",
				Required: true,
				Usage:    "S3 namespace / path prefix",
			},
			&cli.StringSliceFlag{
				Name:     "key-file",
				Required: true,
				Usage:    "File to hash for cache key (repeatable)",
			},
			&cli.StringSliceFlag{
				Name:     "cache-path",
				Required: true,
				Usage:    "Path to bundle into tarball (repeatable)",
			},
			&cli.StringFlag{
				Name:  "display-name",
				Usage: "Human-readable name for S3 put (defaults to --name)",
			},
			&cli.StringFlag{
				Name:  "script-prefix",
				Value: "./evg-cache-scripts",
				Usage: "Path prefix where runtime scripts live on the agent",
			},
			&cli.StringFlag{
				Name:  "output-file",
				Usage: "Write output to this file instead of stdout",
			},
		},
		Action: runGenerate,
	}
}

func runGenerate(_ context.Context, cmd *cli.Command) error {
	displayName := cmd.String("display-name")
	if displayName == "" {
		displayName = cmd.String("name")
	}
	cfg, err := buildConfig(cacheParams{
		name:         cmd.String("name"),
		bucket:       cmd.String("bucket"),
		namespace:    cmd.String("namespace"),
		keyFiles:     cmd.StringSlice("key-file"),
		cachePaths:   cmd.StringSlice("cache-path"),
		scriptPrefix: cmd.String("script-prefix"),
	})
	if err != nil {
		return err
	}
	scriptsFS, err := fs.Sub(scripts.FS, "data")
	if err != nil {
		return fmt.Errorf("opening embedded scripts: %w", err)
	}
	outputFile := cmd.String("output-file")
	includeSetup, err := shouldIncludeSetup(outputFile, generate.SetupFunctionName(cfg))
	if err != nil {
		return err
	}
	if outputFile != "" {
		f, err := os.Create(outputFile)
		if err != nil {
			return fmt.Errorf("creating output file %q: %w", outputFile, err)
		}
		defer f.Close()
		return generate.GenerateFunctions(cfg, displayName, scriptsFS, includeSetup, f)
	}
	return generate.GenerateFunctions(cfg, displayName, scriptsFS, includeSetup, os.Stdout)
}

// shouldIncludeSetup reports whether the setup function should be included in
// the generated output. It returns false when outputFile already exists and
// already defines setupName, so that regenerating a file does not produce a
// duplicate function definition.
func shouldIncludeSetup(outputFile, setupName string) (bool, error) {
	if outputFile == "" {
		// Writing to stdout: we have no existing file to check against.
		return true, nil
	}
	data, err := os.ReadFile(outputFile)
	if errors.Is(err, os.ErrNotExist) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("reading existing output file %q: %w", outputFile, err)
	}
	var existing struct {
		Functions map[string]any `yaml:"functions"`
	}
	if err := yaml.Unmarshal(data, &existing); err != nil {
		// Unrecognizable file — include setup and let the encoder overwrite it.
		return true, nil
	}
	_, alreadyDefined := existing.Functions[setupName]
	return !alreadyDefined, nil
}

func extractScriptsCmd() *cli.Command {
	return &cli.Command{
		Name:  "extract-scripts",
		Usage: "Write runtime scripts to disk for use in Evergreen tasks",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "output-dir",
				Required: true,
				Usage:    "Directory to write scripts into",
			},
		},
		Action: runExtractScripts,
	}
}

func runExtractScripts(_ context.Context, cmd *cli.Command) error {
	outputDir := cmd.String("output-dir")
	if err := extractscripts.ExtractScripts(scripts.FS, outputDir); err != nil {
		return fmt.Errorf("extracting scripts to %q: %w", outputDir, err)
	}
	fmt.Fprintf(os.Stderr, "Scripts written to %s\n", outputDir)
	return nil
}

type cacheParams struct {
	name         string
	bucket       string
	namespace    string
	keyFiles     []string
	cachePaths   []string
	scriptPrefix string
}

func buildConfig(p cacheParams) (cache.Config, error) {
	b, err := cache.NewBuilder(p.name)
	if err != nil {
		return cache.Config{}, err
	}
	cfg := b.
		WithBucket(p.bucket).
		WithNamespace(p.namespace).
		WithKeyFiles(p.keyFiles).
		WithCachePaths(p.cachePaths).
		WithScriptPrefix(p.scriptPrefix).
		Build()
	return cfg, nil
}
