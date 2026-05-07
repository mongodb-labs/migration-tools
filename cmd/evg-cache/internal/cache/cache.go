// Package cache generates Evergreen CI commands for S3-backed artifact caching.
package cache

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/evergreen-ci/shrub"
	"github.com/mongodb-labs/migration-tools/evergreen/s3"
	"github.com/mongodb-labs/migration-tools/evergreen/subprocessexec"
)

var validName = regexp.MustCompile(`^[a-zA-Z0-9-]+$`)

const (
	distroIDExpansionFile = "distro-id.yml"
	cacheKeyExpansionFile = "cache-key.yml"
)

// Config defines how a cache artifact is identified and stored in S3.
// Use NewBuilder to construct. All script paths use the ScriptPrefix field.
type Config struct {
	Bucket            string
	Namespace         string
	Name              string
	Artifact          string
	CacheHitExpansion string
	ScriptPrefix      string
	KeyFiles          []string
	CachePaths        []string
}

// Builder constructs a Config, validating that all required fields are set.
type Builder struct {
	config Config
}

// NewBuilder starts a new Builder for the named cache.
// Returns an error if name does not match [a-zA-Z0-9-]+.
func NewBuilder(name string) (*Builder, error) {
	if !validName.MatchString(name) {
		return nil, fmt.Errorf("cache.Builder: Name %#q must match [a-zA-Z0-9-]+", name)
	}
	return &Builder{config: Config{Name: name, ScriptPrefix: "./evg-cache-scripts"}}, nil
}

func (b *Builder) WithBucket(bucket string) *Builder {
	b.config.Bucket = bucket
	return b
}

func (b *Builder) WithNamespace(namespace string) *Builder {
	b.config.Namespace = namespace
	return b
}

func (b *Builder) WithKeyFiles(files []string) *Builder {
	b.config.KeyFiles = files
	return b
}

func (b *Builder) WithCachePaths(paths []string) *Builder {
	b.config.CachePaths = paths
	return b
}

// WithScriptPrefix sets the path prefix for runtime scripts (relative to ${workdir}).
// Defaults to ./evg-cache-scripts if not set.
func (b *Builder) WithScriptPrefix(prefix string) *Builder {
	b.config.ScriptPrefix = prefix
	return b
}

// Build validates all required fields and returns the Config.
// Panics on missing required fields so misconfiguration is caught early.
func (b *Builder) Build() Config {
	switch {
	case b.config.Bucket == "":
		panic("cache.Builder: Bucket is required")
	case b.config.Namespace == "":
		panic("cache.Builder: Namespace is required")
	case len(b.config.KeyFiles) == 0:
		panic("cache.Builder: KeyFiles is required")
	case len(b.config.CachePaths) == 0:
		panic("cache.Builder: CachePaths is required")
	case b.config.ScriptPrefix == "":
		panic("cache.Builder: ScriptPrefix must not be empty")
	}
	b.config.Artifact = b.config.Name + ".txz"
	b.config.CacheHitExpansion = strings.ReplaceAll(b.config.Name, "-", "_") + "_cache_hit"
	return b.config
}

// S3Path returns the S3 remote file path with runtime expansion variables.
// The namespace is used directly as the S3 path prefix (no project prefix added).
func (c *Config) S3Path() string {
	return fmt.Sprintf("%s/${distro_id}/${cache_key}/%s", c.Namespace, c.Artifact)
}

// scriptPath returns the full path to a script using the configured ScriptPrefix.
// Uses string concatenation (not filepath.Join) to preserve a leading "./" prefix.
func (c *Config) scriptPath(script string) string {
	return strings.TrimRight(c.ScriptPrefix, "/") + "/" + script
}

// ComputeKeyCommands returns commands that set ${distro_id} and ${cache_key} expansions.
func (c *Config) ComputeKeyCommands() []*shrub.CommandDefinition {
	keyArgs := []string{c.scriptPath("compute-cache-key.py")}
	for _, f := range c.KeyFiles {
		keyArgs = append(keyArgs, "--file", f)
	}
	keyArgs = append(keyArgs, "--output", filepath.Join("${workdir}", cacheKeyExpansionFile))

	return []*shrub.CommandDefinition{
		subprocessexec.NewCmdBuilder(c.scriptPath("set-distro-id-expansion.sh")).
			WithArgs("${workdir}").
			Build(),

		shrub.CmdExpansionsUpdate{
			File: filepath.Join("${workdir}", distroIDExpansionFile),
		}.Resolve(),

		subprocessexec.NewCmdBuilder(c.scriptPath("run-python-script.sh")).
			WithArgs(keyArgs...).
			WithWorkingDirectory("${workdir}").
			Build(),

		shrub.CmdExpansionsUpdate{
			File: filepath.Join("${workdir}", cacheKeyExpansionFile),
		}.Resolve(),
	}
}

// RestoreCommands returns commands to download and extract the cached artifact.
// A cache miss sets the cache-hit expansion to "" without failing the task.
func (c *Config) RestoreCommands() []*shrub.CommandDefinition {
	expansionName := c.CacheHitExpansion
	expansionFile := filepath.Join("${workdir}", expansionName+".yml")

	return []*shrub.CommandDefinition{
		s3.NewGetCmdBuilder(c.S3Path()).
			WithBucket(c.Bucket).
			WithLocalFile(c.Artifact).
			WithIsOptional().
			Build(),

		subprocessexec.NewCmdBuilder(c.scriptPath("run-python-script.sh")).
			WithArgs(
				c.scriptPath("restore-cache-artifact.py"),
				"--artifact", c.Artifact,
				"--output", expansionFile,
				"--expansion-name", expansionName,
			).
			WithWorkingDirectory("${workdir}").
			Build(),

		shrub.CmdExpansionsUpdate{
			File: expansionFile,
		}.Resolve(),
	}
}

// SaveCommands returns commands to create the artifact tarball (on a cache miss)
// and upload it to S3.
func (c *Config) SaveCommands(displayName string) []*shrub.CommandDefinition {
	createArgs := []string{
		c.scriptPath("create-cache-artifact.py"),
		"--artifact", c.Artifact,
		"--cache-hit-expansion", c.CacheHitExpansion,
	}
	for _, p := range c.CachePaths {
		createArgs = append(createArgs, "--path", p)
	}

	return []*shrub.CommandDefinition{
		subprocessexec.NewCmdBuilder(c.scriptPath("run-python-script.sh")).
			WithArgs(createArgs...).
			WithIncludeExpansionsInEnv(c.CacheHitExpansion).
			WithWorkingDirectory("${workdir}").
			Build(),

		s3.NewPutCmdBuilder().
			WithBucket(c.Bucket).
			WithRemoteFile(c.S3Path()).
			WithLocalFile(c.Artifact).
			WithContentType(s3.BinaryContentType).
			WithDisplayName(displayName).
			WithIsOptional().
			WithSkipExisting().
			Build(),
	}
}
