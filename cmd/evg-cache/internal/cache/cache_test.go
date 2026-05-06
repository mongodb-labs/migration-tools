package cache_test

import (
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/mongodb-labs/migration-tools/cmd/evg-cache/internal/cache"
	"github.com/stretchr/testify/suite"

	"github.com/evergreen-ci/shrub"
)

type CacheTestSuite struct {
	suite.Suite
}

func TestCacheTestSuite(t *testing.T) {
	suite.Run(t, new(CacheTestSuite))
}

func testConfig() cache.Config {
	b, err := cache.NewBuilder("mise-and-go")
	if err != nil {
		panic(err) // "mise-and-go" is always a valid name
	}
	return b.
		WithBucket("mciuploads").
		WithNamespace("myproject/mise-cache").
		WithKeyFiles([]string{"mise.toml", "mise-version.txt"}).
		WithCachePaths([]string{".local/bin/mise", ".local/share/mise"}).
		WithScriptDir("./evg-cache-scripts").
		Build()
}

func (s *CacheTestSuite) mustMarshal(cmds []*shrub.CommandDefinition) string {
	b, err := yaml.Marshal(shrub.CommandSequence(cmds))
	s.Require().NoError(err, "marshaling command sequence to YAML should not fail")
	return string(b)
}

func (s *CacheTestSuite) TestS3Path_UsesNamespaceDirectly() {
	cfg := testConfig()
	s.Assert().Equal(
		"myproject/mise-cache/${distro_id}/${cache_key}/mise-and-go.txz",
		cfg.S3Path(),
		"S3 path should use namespace directly without hardcoded project prefix",
	)
}

func (s *CacheTestSuite) TestS3Path_ReflectsNamespaceAndArtifact() {
	b, err := cache.NewBuilder("tools")
	s.Require().NoError(err)
	cfg := b.
		WithBucket("mciuploads").
		WithNamespace("other/cache").
		WithKeyFiles([]string{"go.sum"}).
		WithCachePaths([]string{".gomodcache"}).
		WithScriptDir("./scripts").
		Build()
	s.Assert().Equal(
		"other/cache/${distro_id}/${cache_key}/tools.txz",
		cfg.S3Path(),
		"S3 path namespace and artifact should reflect Config fields",
	)
}

func (s *CacheTestSuite) TestNewBuilder_ReturnsErrorOnInvalidName() {
	_, err := cache.NewBuilder("invalid name!")
	s.Assert().
		Error(err, "NewBuilder should return an error when name contains characters outside [a-zA-Z0-9-]")
}

func (s *CacheTestSuite) TestBuild_PanicsWhenBucketMissing() {
	b, err := cache.NewBuilder("foo")
	s.Require().NoError(err)
	s.Assert().Panics(func() {
		b.WithNamespace("ns").
			WithKeyFiles([]string{"f"}).
			WithCachePaths([]string{"p"}).
			Build()
	}, "Build should panic when Bucket is not set")
}

func (s *CacheTestSuite) TestBuild_PanicsMissingNamespace() {
	b, err := cache.NewBuilder("foo")
	s.Require().NoError(err)
	s.Assert().Panics(func() {
		b.WithBucket("b").
			WithKeyFiles([]string{"f"}).
			WithCachePaths([]string{"p"}).
			Build()
	}, "Build should panic when Namespace is not set")
}

func (s *CacheTestSuite) TestBuild_PanicsWhenKeyFilesMissing() {
	b, err := cache.NewBuilder("foo")
	s.Require().NoError(err)
	s.Assert().Panics(func() {
		b.WithBucket("b").
			WithNamespace("ns").
			WithCachePaths([]string{"p"}).
			Build()
	}, "Build should panic when KeyFiles is not set")
}

func (s *CacheTestSuite) TestBuild_PanicsWhenCachePathsMissing() {
	b, err := cache.NewBuilder("foo")
	s.Require().NoError(err)
	s.Assert().Panics(func() {
		b.WithBucket("b").
			WithNamespace("ns").
			WithKeyFiles([]string{"f"}).
			Build()
	}, "Build should panic when CachePaths is not set")
}

func (s *CacheTestSuite) TestBuild_PanicsWhenScriptDirEmpty() {
	b, err := cache.NewBuilder("foo")
	s.Require().NoError(err)
	s.Assert().Panics(func() {
		b.WithBucket("b").
			WithNamespace("ns").
			WithKeyFiles([]string{"f"}).
			WithCachePaths([]string{"p"}).
			WithScriptDir("").
			Build()
	}, "Build should panic when ScriptDir is explicitly set to empty string")
}

func (s *CacheTestSuite) TestCacheHitExpansion_DerivedFromName() {
	b, err := cache.NewBuilder("mise-and-go")
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

func (s *CacheTestSuite) TestComputeKeyCommands_ContainsAllKeyFiles() {
	cfg := testConfig()
	y := s.mustMarshal(cfg.ComputeKeyCommands())
	for _, f := range cfg.KeyFiles {
		s.Assert().Contains(y, f, "compute-cache-key.py args should include each KeyFile")
	}
}

func (s *CacheTestSuite) TestComputeKeyCommands_ContainsKeyExpansions() {
	b, err := cache.NewBuilder("mise-and-go")
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
	s.Assert().Contains(
		y,
		"${variant}",
		"compute-cache-key.py args should include each key expansion value",
	)
	s.Assert().Contains(
		y,
		"${os}",
		"compute-cache-key.py args should include each key expansion value",
	)
}

func (s *CacheTestSuite) TestComputeKeyCommands_NoKeyExpansionsByDefault() {
	cfg := testConfig()
	y := s.mustMarshal(cfg.ComputeKeyCommands())
	s.Assert().NotContains(
		y,
		"--key-expansion",
		"compute-cache-key.py args should not include --key-expansion when none are configured",
	)
}

func (s *CacheTestSuite) TestComputeKeyCommands_UsesScriptDir() {
	cfg := testConfig()
	y := s.mustMarshal(cfg.ComputeKeyCommands())
	s.Assert().Contains(
		y,
		"./evg-cache-scripts/set-distro-id-expansion.sh",
		"set-distro-id script path should use the configured script dir",
	)
	s.Assert().Contains(
		y,
		"./evg-cache-scripts/compute-cache-key.py",
		"compute-cache-key script path should use the configured script dir",
	)
}

func (s *CacheTestSuite) TestRestoreCommands_S3PathInGetCommand() {
	cfg := testConfig()
	y := s.mustMarshal(cfg.RestoreCommands())
	s.Assert().Contains(
		y,
		cfg.S3Path(),
		"s3.get remote_file should use the config's S3 path",
	)
}

func (s *CacheTestSuite) TestRestoreCommands_S3GetIsOptional() {
	cfg := testConfig()
	y := s.mustMarshal(cfg.RestoreCommands())
	s.Assert().Contains(
		y,
		"optional: true",
		"s3.get should always be optional so a cache miss does not fail the task",
	)
}

func (s *CacheTestSuite) TestRestoreCommands_CacheHitExpansion() {
	cfg := testConfig()
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

func (s *CacheTestSuite) TestRestoreCommands_UsesScriptDir() {
	cfg := testConfig()
	y := s.mustMarshal(cfg.RestoreCommands())
	s.Assert().Contains(y, "./evg-cache-scripts/restore-cache-artifact.py",
		"restore-cache-artifact script path should use the configured script dir")
}

func (s *CacheTestSuite) TestSaveCommands_CreatesTarballWithCachePaths() {
	cfg := testConfig()
	y := s.mustMarshal(cfg.SaveCommands("mise and go executables"))
	for _, p := range cfg.CachePaths {
		s.Assert().Contains(y, p, "tar command should include each CachePath")
	}
}

func (s *CacheTestSuite) TestSaveCommands_PassesCacheHitExpansionToScript() {
	cfg := testConfig()
	y := s.mustMarshal(cfg.SaveCommands("mise and go executables"))
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
	cfg := testConfig()
	y := s.mustMarshal(cfg.SaveCommands("mise and go executables"))
	s.Assert().Contains(
		y,
		cfg.S3Path(),
		"s3.put remote_file should use the config's S3 path",
	)
}

func (s *CacheTestSuite) TestSaveCommands_SkipExisting() {
	cfg := testConfig()
	y := s.mustMarshal(cfg.SaveCommands("mise and go executables"))
	s.Assert().Contains(
		y,
		"skip_existing",
		"s3.put should set skip_existing to avoid overwriting a populated cache",
	)
}

func (s *CacheTestSuite) TestSaveCommands_UsesScriptDir() {
	cfg := testConfig()
	y := s.mustMarshal(cfg.SaveCommands("mise and go executables"))
	s.Assert().Contains(y, "./evg-cache-scripts/create-cache-artifact.py",
		"create-cache-artifact script path should use the configured script dir")
}
