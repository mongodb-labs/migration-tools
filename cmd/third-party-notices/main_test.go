package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLicenseRegexp1(t *testing.T) {
	matches := []string{
		"LICENSE",
		"license",
		"License",
		"LICENCE",
		"licence",
		"Licence",
		"LICENSE.md",
		"LICENSE.txt",
		"licence.html",
	}
	for _, name := range matches {
		assert.True(t, licenseRegexp1.MatchString(name), "should match %q", name)
	}

	noMatches := []string{
		"README.md",
		"NOTICE",
		"COPYING",
		"my-LICENSE",
		"LICENSE-MIT",
	}
	for _, name := range noMatches {
		assert.False(t, licenseRegexp1.MatchString(name), "should not match %q", name)
	}
}

func TestLicenseRegexp2(t *testing.T) {
	matches := []string{
		"MIT.license",
		"something.LICENSE",
		"foo.licence",
		"bar.LICENCE",
	}
	for _, name := range matches {
		assert.True(t, licenseRegexp2.MatchString(name), "should match %q", name)
	}

	noMatches := []string{
		"LICENSE",
		"README.md",
		"license-MIT",
	}
	for _, name := range noMatches {
		assert.False(t, licenseRegexp2.MatchString(name), "should not match %q", name)
	}
}

func TestLicenseFiles(t *testing.T) {
	root := t.TempDir()
	vendorDir := filepath.Join(root, "vendor")

	mkFile := func(t *testing.T, path string) {
		t.Helper()
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
		require.NoError(t, os.WriteFile(path, []byte("license text"), 0644))
	}

	// License files that should be found.
	mkFile(t, filepath.Join(vendorDir, "github.com/foo/bar", "LICENSE"))
	mkFile(t, filepath.Join(vendorDir, "github.com/foo/bar", "subpkg", "LICENSE.md"))
	mkFile(t, filepath.Join(vendorDir, "github.com/baz/qux", "LICENCE"))
	mkFile(t, filepath.Join(vendorDir, "github.com/baz/qux", "MIT.license"))

	// Non-license files that should be ignored.
	mkFile(t, filepath.Join(vendorDir, "github.com/foo/bar", "README.md"))
	mkFile(t, filepath.Join(vendorDir, "github.com/baz/qux", "NOTICE"))

	got, err := licenseFiles(root)
	require.NoError(t, err)

	var gotPkgs []string
	for _, lf := range got {
		gotPkgs = append(gotPkgs, lf.packageName+"/"+filepath.Base(lf.path))
	}

	// Results should be sorted by path (which sorts packages alphabetically since vendor/ prefix is shared).
	assert.Equal(t, []string{
		"github.com/baz/qux/LICENCE",
		"github.com/baz/qux/MIT.license",
		"github.com/foo/bar/LICENSE",
		"github.com/foo/bar/subpkg/LICENSE.md",
	}, gotPkgs)
}

func TestLicenseFilesEmpty(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "vendor"), 0755))

	got, err := licenseFiles(root)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestLicenseFilesPackageName(t *testing.T) {
	root := t.TempDir()
	vendorDir := filepath.Join(root, "vendor")

	path := filepath.Join(vendorDir, "golang.org/x/sync", "LICENSE")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
	require.NoError(t, os.WriteFile(path, []byte("Apache 2.0"), 0644))

	got, err := licenseFiles(root)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "golang.org/x/sync", got[0].packageName)
	assert.Equal(t, path, got[0].path)
}
