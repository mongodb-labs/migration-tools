// cmd/evg-cache/main.go
package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"

	"github.com/mongodb-labs/migration-tools/cmd/evg-cache/internal/cache"
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
			&cli.StringSliceFlag{
				Name:  "key-expansion",
				Usage: "Evergreen expansion value to include in the cache key hash (repeatable)",
			},
			&cli.StringFlag{
				Name:  "display-name",
				Usage: "Human-readable name for S3 put (defaults to --name)",
			},
			&cli.StringFlag{
				Name:  "script-dir",
				Value: "./evg-cache-scripts",
				Usage: "Directory where runtime scripts live on the agent",
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
		name:          cmd.String("name"),
		bucket:        cmd.String("bucket"),
		namespace:     cmd.String("namespace"),
		keyFiles:      cmd.StringSlice("key-file"),
		keyExpansions: cmd.StringSlice("key-expansion"),
		cachePaths:    cmd.StringSlice("cache-path"),
		scriptDir:     cmd.String("script-dir"),
	})
	if err != nil {
		return err
	}
	scriptsFS, err := fs.Sub(scripts.FS, "data")
	if err != nil {
		return fmt.Errorf("opening embedded scripts: %w", err)
	}
	outputFile := cmd.String("output-file")
	if outputFile != "" {
		f, err := os.Create(outputFile)
		if err != nil {
			return fmt.Errorf("creating output file %q: %w", outputFile, err)
		}
		defer f.Close()
		return generate.GenerateFunctions(cfg, displayName, scriptsFS, f)
	}
	return generate.GenerateFunctions(cfg, displayName, scriptsFS, os.Stdout)
}

type cacheParams struct {
	name          string
	bucket        string
	namespace     string
	keyFiles      []string
	keyExpansions []string
	cachePaths    []string
	scriptDir     string
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
		WithKeyExpansions(p.keyExpansions).
		WithCachePaths(p.cachePaths).
		WithScriptDir(p.scriptDir).
		Build()
	return cfg, nil
}
