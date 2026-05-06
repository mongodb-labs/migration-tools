// Package extractscripts writes the embedded evg-cache runtime scripts to disk.
package extractscripts

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ExtractScripts writes all files from the embedded FS (rooted at "data/")
// to outDir, creating it if it doesn't exist. Shell scripts (.sh) are written
// with executable permissions (0755); Python scripts get 0644.
func ExtractScripts(embedded fs.FS, outDir string) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}

	return fs.WalkDir(embedded, "data", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		content, readErr := fs.ReadFile(embedded, path)
		if readErr != nil {
			return readErr
		}

		name := filepath.Base(path)
		dest := filepath.Join(outDir, name)

		perm := fs.FileMode(0o644)
		if strings.HasSuffix(name, ".sh") {
			perm = 0o755
		}

		return os.WriteFile(dest, content, perm)
	})
}
