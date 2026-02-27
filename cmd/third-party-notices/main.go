package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/mongodb-labs/migration-tools/git"
)

//nolint:misspell // "license" is intentional here
var (
	// This matches a file that starts with "license" or "licence", in any case, with an optional
	// extension.
	licenseRegexp1 = regexp.MustCompile(`(?i)^licen[cs]e(?:\..+)?$`)
	// This matches a file that has an extension of "license" or "licence", in any case.
	licenseRegexp2      = regexp.MustCompile(`(?i)\.licen[cs]e$`)
	trailingSpaceRegexp = regexp.MustCompile(`(?m)[^\n\S]+$`)

	horizontalLine = strings.Repeat("-", 70)
)

func main() {
	if err := writeThirdPartyNotices(); err != nil {
		fmt.Fprintf(os.Stderr, "%s", err)
		os.Exit(1)
	}
}

// writeThirdPartyNotices writes the `THIRD-PARTY-NOTICES` file for a project, which contains all
// the licenses for vendored code in the project.
func writeThirdPartyNotices() error {
	root, err := git.FindRepoRoot(context.Background())
	if err != nil {
		return err
	}

	licenses, err := licenseFiles(root)
	if err != nil {
		return err
	}

	var notices string
	for _, lf := range licenses {
		if notices != "" {
			notices += "\n"
		}
		notices += horizontalLine
		notices += "\n"
		notices += fmt.Sprintf(
			"License notice for %s (%s)\n",
			lf.packageName,
			filepath.Base(lf.path),
		)
		notices += horizontalLine
		notices += "\n"
		notices += "\n"

		content, err := os.ReadFile(lf.path)
		if err != nil {
			return fmt.Errorf("read license file %q: %w", lf.path, err)
		}

		contentStr := string(content)

		// Trim trailing space from each line.
		contentStr = trailingSpaceRegexp.ReplaceAllString(contentStr, "")

		notices += contentStr
	}

	return os.WriteFile(filepath.Join(root, "THIRD-PARTY-NOTICES"), []byte(notices), 0644)
}

const vendorDir string = "vendor"

type licenseFile struct {
	packageName string
	path        string
}

func licenseFiles(root string) ([]licenseFile, error) {
	var (
		walkIn       = filepath.Join(root, vendorDir)
		pathPrefix   = walkIn + "/"
		licenseFiles []licenseFile
	)
	err := filepath.WalkDir(
		walkIn,
		func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if !d.Type().IsRegular() {
				return nil
			}

			filename := d.Name()

			if licenseRegexp1.MatchString(filename) || licenseRegexp2.MatchString(filename) {
				packageName := strings.TrimPrefix(filepath.Dir(path), pathPrefix)
				licenseFiles = append(licenseFiles, licenseFile{packageName, path})
			}

			return nil
		},
	)
	if err != nil {
		return nil, err
	}

	sort.Slice(
		licenseFiles,
		func(i, j int) bool {
			return licenseFiles[i].path < licenseFiles[j].path
		},
	)

	return licenseFiles, nil
}
