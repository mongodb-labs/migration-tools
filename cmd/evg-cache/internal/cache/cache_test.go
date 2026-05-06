package cache

import (
	"bytes"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/evergreen-ci/shrub"
	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/suite"
)

type CacheTestSuite struct {
	suite.Suite
}

func TestCacheTestSuite(t *testing.T) {
	suite.Run(t, new(CacheTestSuite))
}

func (s *CacheTestSuite) TestS3Path_UsesNamespaceDirectly() {
	cfg := s.sampleConfig()
	s.Assert().Equal(
		"myproject/mise-cache/${distro_id}/${cache_key}/mise-and-go.txz",
		cfg.S3Path(),
		"S3 path should use namespace directly without hardcoded project prefix",
	)
}

func (s *CacheTestSuite) TestS3Path_ReflectsNamespaceAndArtifact() {
	b, err := NewBuilder("tools")
	s.Require().NoError(err)
	cfg := b.
		WithBucket("mciuploads").
		WithNamespace("other/cache").
		WithKeyFiles([]string{"go.sum"}).
		WithCachePaths([]string{".gomodcache"}).
		Build()
	s.Assert().Equal(
		"other/cache/${distro_id}/${cache_key}/tools.txz",
		cfg.S3Path(),
		"S3 path namespace and artifact should reflect Config fields",
	)
}

func (s *CacheTestSuite) TestS3Path_TrailingSlashInNamespaceIsNormalized() {
	b, err := NewBuilder("tools")
	s.Require().NoError(err)
	cfg := b.
		WithBucket("mciuploads").
		WithNamespace("myproject/go-cache/").
		WithKeyFiles([]string{"go.sum"}).
		WithCachePaths([]string{".gomodcache"}).
		Build()
	s.Assert().Equal(
		"myproject/go-cache/${distro_id}/${cache_key}/tools.txz",
		cfg.S3Path(),
		"trailing slash in namespace should be stripped so the S3 path has no double-slash",
	)
}

func (s *CacheTestSuite) TestNewBuilder_ReturnsErrorOnInvalidName() {
	_, err := NewBuilder("invalid name!")
	s.Assert().
		Error(err, "NewBuilder should return an error when name contains characters outside [a-zA-Z0-9-]")
}

func (s *CacheTestSuite) TestBuild_PanicsWhenBucketMissing() {
	b, err := NewBuilder("foo")
	s.Require().NoError(err)
	s.Assert().Panics(func() {
		b.WithNamespace("ns").
			WithKeyFiles([]string{"f"}).
			WithCachePaths([]string{"p"}).
			Build()
	}, "Build should panic when Bucket is not set")
}

func (s *CacheTestSuite) TestBuild_PanicsMissingNamespace() {
	b, err := NewBuilder("foo")
	s.Require().NoError(err)
	s.Assert().Panics(func() {
		b.WithBucket("b").
			WithKeyFiles([]string{"f"}).
			WithCachePaths([]string{"p"}).
			Build()
	}, "Build should panic when Namespace is not set")
}

func (s *CacheTestSuite) TestBuild_PanicsWhenKeyFilesMissing() {
	b, err := NewBuilder("foo")
	s.Require().NoError(err)
	s.Assert().Panics(func() {
		b.WithBucket("b").
			WithNamespace("ns").
			WithCachePaths([]string{"p"}).
			Build()
	}, "Build should panic when KeyFiles is not set")
}

func (s *CacheTestSuite) TestBuild_PanicsWhenCachePathsMissing() {
	b, err := NewBuilder("foo")
	s.Require().NoError(err)
	s.Assert().Panics(func() {
		b.WithBucket("b").
			WithNamespace("ns").
			WithKeyFiles([]string{"f"}).
			Build()
	}, "Build should panic when CachePaths is not set")
}

func (s *CacheTestSuite) TestCacheHitExpansion_DerivedFromName() {
	b, err := NewBuilder("mise-and-go")
	s.Require().NoError(err)
	cfg := b.
		WithBucket("b").
		WithNamespace("ns").
		WithKeyFiles([]string{"f"}).
		WithCachePaths([]string{"p"}).
		Build()
	s.Assert().Equal(
		"mise_and_go_cache_hit",
		cfg.CacheHitExpansion,
		"CacheHitExpansion should replace hyphens with underscores and append _cache_hit",
	)
}

func (s *CacheTestSuite) TestComputeKeyCommands_ContainsKeyExpansions() {
	b, err := NewBuilder("mise-and-go")
	s.Require().NoError(err)
	cfg := b.
		WithBucket("mciuploads").
		WithNamespace("myproject/mise-cache").
		WithKeyFiles([]string{"mise.toml"}).
		WithKeyExpansions([]string{"${variant}", "${os}"}).
		WithCachePaths([]string{".local/bin/mise"}).
		Build()
	y := s.mustMarshal(cfg.ComputeKeyCommands())
	s.Assert().Contains(
		y,
		"--key-expansion",
		"compute-cache-key.py args should include --key-expansion flag when KeyExpansions are set",
	)
}

func (s *CacheTestSuite) TestComputeKeyCommands_NoKeyExpansionsByDefault() {
	cfg := s.sampleConfig()
	y := s.mustMarshal(cfg.ComputeKeyCommands())
	s.Assert().NotContains(
		y,
		"--key-expansion",
		"compute-cache-key.py args should not include --key-expansion when none are configured",
	)
}

func (s *CacheTestSuite) TestRestoreCommands_S3PathInGetCommand() {
	cfg := s.sampleConfig()
	y := s.mustMarshal(cfg.RestoreCommands())
	s.Assert().Contains(
		y,
		cfg.S3Path(),
		"s3.get remote_file should use the config's S3 path",
	)
}

func (s *CacheTestSuite) TestRestoreCommands_S3GetIsOptional() {
	cfg := s.sampleConfig()
	y := s.mustMarshal(cfg.RestoreCommands())
	s.Assert().Contains(
		y,
		"optional: true",
		"s3.get should always be optional so a cache miss does not fail the task",
	)
}

func (s *CacheTestSuite) TestRestoreCommands_CacheHitExpansion() {
	cfg := s.sampleConfig()
	y := s.mustMarshal(cfg.RestoreCommands())
	s.Assert().Contains(
		y,
		"${workdir}/"+cfg.CacheHitExpansion+".yml",
		"expansion file should use the configured CacheHitExpansion name",
	)
	s.Assert().Contains(
		y,
		cfg.CacheHitExpansion,
		"restore-cache-artifact.py should receive the configured expansion name",
	)
}

func (s *CacheTestSuite) TestSaveCommands_PassesCacheHitExpansionToScript() {
	cfg := s.sampleConfig()
	y := s.mustMarshal(cfg.SaveCommands())
	s.Assert().Contains(
		y,
		"--cache-hit-expansion",
		"create-cache-artifact.py should receive --cache-hit-expansion so it can skip tarball creation on a cache hit",
	)
	s.Assert().Contains(
		y,
		cfg.CacheHitExpansion,
		"create-cache-artifact.py should receive the configured cache hit expansion name",
	)
}

func (s *CacheTestSuite) TestSaveCommands_S3PathInPutCommand() {
	cfg := s.sampleConfig()
	y := s.mustMarshal(cfg.SaveCommands())
	s.Assert().Contains(
		y,
		cfg.S3Path(),
		"s3.put remote_file should use the config's S3 path",
	)
}

func (s *CacheTestSuite) TestSaveCommands_SkipExisting() {
	cfg := s.sampleConfig()
	y := s.mustMarshal(cfg.SaveCommands())
	s.Assert().Contains(
		y,
		"skip_existing",
		"s3.put should set skip_existing to avoid overwriting a populated cache",
	)
}

func (s *CacheTestSuite) TestDisplayName_DefaultsToName() {
	cfg := s.sampleConfig()
	s.Assert().Equal(
		cfg.Name,
		cfg.DisplayName,
		"DisplayName should default to Name when WithDisplayName is not called",
	)
}

func (s *CacheTestSuite) TestDisplayName_WithDisplayNameOverridesDefault() {
	b, err := NewBuilder("mise-and-go")
	s.Require().NoError(err)
	cfg := b.
		WithBucket("mciuploads").
		WithNamespace("myproject/mise-cache").
		WithKeyFiles([]string{"mise.toml"}).
		WithCachePaths([]string{".local/bin/mise"}).
		WithDisplayName("mise and go executables").
		Build()
	s.Assert().Equal(
		"mise and go executables",
		cfg.DisplayName,
		"DisplayName should use the value passed to WithDisplayName",
	)
}

func (s *CacheTestSuite) TestHeaderComment() {
	header := "This file was generated by running `evg-cache generate --name foo`. Do not edit this file directly!"
	output := s.generate(s.sampleConfig(), header, false)
	s.Assert().Contains(output, "# "+header,
		"output should include the complete header text as a YAML comment")
	s.Assert().True(len(output) > 0 && output[0] == '#',
		"header comment should appear at the very start of the output")
}

func (s *CacheTestSuite) TestNoHeaderComment() {
	output := s.generate(s.sampleConfig(), "", false)
	// Without a header the output should start directly with the YAML content, not a comment line.
	s.Assert().True(strings.HasPrefix(output, "functions:"),
		"output should start with 'functions:' when no header is provided")
}

func (s *CacheTestSuite) TestSetupScriptsCommands_ErrorOnDuplicateBasename() {
	dupFS := fstest.MapFS{
		"sub/foo.sh":   {Data: []byte("#!/bin/bash\n")},
		"other/foo.sh": {Data: []byte("#!/bin/bash\n")},
	}
	_, err := setupScriptsCommands(dupFS, "./scripts")
	s.Require().Error(err, "duplicate basenames in the scripts FS should produce an error")
	s.Assert().Contains(err.Error(), "foo.sh",
		"error should name the colliding basename")
}

func (s *CacheTestSuite) sampleConfig() Config {
	b, err := NewBuilder("mise-and-go")
	s.Require().NoError(err)
	return b.
		WithBucket("mciuploads").
		WithNamespace("myproject/mise-cache").
		WithKeyFiles([]string{"mise.toml", "mise-version.txt"}).
		WithCachePaths([]string{".local/bin/mise", ".local/share/mise"}).
		Build()
}

func (s *CacheTestSuite) mustMarshal(cmds shrub.CommandSequence) string {
	b, err := yaml.Marshal(cmds)
	s.Require().NoError(err, "marshaling command sequence to YAML should not fail")
	return string(b)
}

func (s *CacheTestSuite) generate(cfg Config, header string, omitSetup bool) string {
	var buf bytes.Buffer
	s.Require().NoError(
		cfg.GenerateFunctions(header, omitSetup, &buf),
		"GenerateFunctions should not return an error",
	)
	return buf.String()
}
