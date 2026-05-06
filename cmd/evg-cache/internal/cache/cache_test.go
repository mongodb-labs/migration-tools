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
	return cache.NewBuilder("mise-and-go").
		WithBucket("mciuploads").
		WithNamespace("myproject/mise-cache").
		WithKeyFiles([]string{"mise.toml", "mise-version.txt"}).
		WithCachePaths([]string{".local/bin/mise", ".local/share/mise"}).
		WithScriptPrefix("./evg-cache-scripts").
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
	cfg := cache.NewBuilder("tools").
		WithBucket("mciuploads").
		WithNamespace("other/cache").
		WithKeyFiles([]string{"go.sum"}).
		WithCachePaths([]string{".gomodcache"}).
		WithScriptPrefix("./scripts").
		Build()
	s.Assert().Equal(
		"other/cache/${distro_id}/${cache_key}/tools.txz",
		cfg.S3Path(),
		"S3 path namespace and artifact should reflect Config fields",
	)
}

func (s *CacheTestSuite) TestNewBuilder_PanicsOnInvalidName() {
	s.Assert().Panics(func() {
		cache.NewBuilder("invalid name!")
	}, "NewBuilder should panic when name contains characters outside [a-zA-Z0-9-]")
}

func (s *CacheTestSuite) TestBuild_PanicsWhenBucketMissing() {
	s.Assert().Panics(func() {
		cache.NewBuilder("foo").
			WithNamespace("ns").
			WithKeyFiles([]string{"f"}).
			WithCachePaths([]string{"p"}).
			Build()
	}, "Build should panic when Bucket is not set")
}

func (s *CacheTestSuite) TestBuild_PanicsMissingNamespace() {
	s.Assert().Panics(func() {
		cache.NewBuilder("foo").
			WithBucket("b").
			WithKeyFiles([]string{"f"}).
			WithCachePaths([]string{"p"}).
			Build()
	}, "Build should panic when Namespace is not set")
}

func (s *CacheTestSuite) TestBuild_PanicsWhenKeyFilesMissing() {
	s.Assert().Panics(func() {
		cache.NewBuilder("foo").
			WithBucket("b").
			WithNamespace("ns").
			WithCachePaths([]string{"p"}).
			Build()
	}, "Build should panic when KeyFiles is not set")
}

func (s *CacheTestSuite) TestBuild_PanicsWhenCachePathsMissing() {
	s.Assert().Panics(func() {
		cache.NewBuilder("foo").
			WithBucket("b").
			WithNamespace("ns").
			WithKeyFiles([]string{"f"}).
			Build()
	}, "Build should panic when CachePaths is not set")
}

func (s *CacheTestSuite) TestBuild_PanicsWhenScriptPrefixEmpty() {
	s.Assert().Panics(func() {
		cache.NewBuilder("foo").
			WithBucket("b").
			WithNamespace("ns").
			WithKeyFiles([]string{"f"}).
			WithCachePaths([]string{"p"}).
			WithScriptPrefix("").
			Build()
	}, "Build should panic when ScriptPrefix is explicitly set to empty string")
}

func (s *CacheTestSuite) TestCacheHitExpansion_DerivedFromName() {
	cfg := cache.NewBuilder("mise-and-go").
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

func (s *CacheTestSuite) TestComputeKeyCommands_UsesScriptPrefix() {
	cfg := testConfig()
	y := s.mustMarshal(cfg.ComputeKeyCommands())
	s.Assert().Contains(y, "./evg-cache-scripts/set-distro-id-expansion.sh",
		"set-distro-id script path should use the configured script prefix")
	s.Assert().Contains(y, "./evg-cache-scripts/compute-cache-key.py",
		"compute-cache-key script path should use the configured script prefix")
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
		"detect-cache-hit.py should receive the configured expansion name",
	)
}

func (s *CacheTestSuite) TestRestoreCommands_UsesScriptPrefix() {
	cfg := testConfig()
	y := s.mustMarshal(cfg.RestoreCommands())
	s.Assert().Contains(y, "./evg-cache-scripts/detect-cache-hit.py",
		"detect-cache-hit script path should use the configured script prefix")
	s.Assert().Contains(y, "./evg-cache-scripts/extract-cache-artifact.py",
		"extract-cache-artifact script path should use the configured script prefix")
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

func (s *CacheTestSuite) TestSaveCommands_UsesScriptPrefix() {
	cfg := testConfig()
	y := s.mustMarshal(cfg.SaveCommands("mise and go executables"))
	s.Assert().Contains(y, "./evg-cache-scripts/create-cache-artifact.py",
		"create-cache-artifact script path should use the configured script prefix")
}
