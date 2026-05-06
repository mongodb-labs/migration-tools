// Package generate writes Evergreen functions YAML for evg-cache configurations.
package generate

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/evergreen-ci/shrub"
	"github.com/goccy/go-yaml"
	"github.com/mongodb-labs/migration-tools/cmd/evg-cache/internal/cache"
)

// GenerateFunctions writes Evergreen function definitions to w:
//   - restore-<name>-cache: compute key, download artifact, detect hit, extract
//   - save-<name>-cache: create tarball (on miss), upload to S3
//   - setup-<script-dir>: write runtime scripts to ScriptDir; the name is
//     derived from the script dir so multiple caches sharing a dir emit
//     identical copies — the runtime sentinel makes repeated execution a no-op.
//
// scriptsFS must contain the runtime scripts written by the setup function.
// Pass scripts.FS (after fs.Sub to strip the "data/" prefix) from the evg-cache binary.
func GenerateFunctions(
	cfg cache.Config,
	displayName string,
	scriptsFS fs.FS,
	w io.Writer,
) error {
	restoreName := "restore-" + cfg.Name + "-cache"
	saveName := "save-" + cfg.Name + "-cache"

	restoreSeq := shrub.CommandSequence(
		append(cfg.ComputeKeyCommands(), cfg.RestoreCommands()...),
	)
	saveSeq := shrub.CommandSequence(cfg.SaveCommands(displayName))

	setupCmds, err := setupScriptsCommands(scriptsFS, cfg.ScriptDir)
	if err != nil {
		return fmt.Errorf("building setup-scripts commands: %w", err)
	}
	setupSeq := shrub.CommandSequence(setupCmds)

	functions := map[string]*shrub.CommandSequence{
		restoreName:            &restoreSeq,
		saveName:               &saveSeq,
		SetupFunctionName(cfg): &setupSeq,
	}

	out := struct {
		Functions map[string]*shrub.CommandSequence `yaml:"functions"`
	}{
		Functions: functions,
	}

	enc := yaml.NewEncoder(w, yaml.UseLiteralStyleIfMultiline(true))
	if err := enc.Encode(out); err != nil {
		return err
	}
	return enc.Close()
}

// SetupFunctionName returns the Evergreen function name for the setup function
// that corresponds to cfg's ScriptDir (e.g. "setup-evg-cache-scripts").
// Multiple caches that share the same ScriptDir produce the same name, so
// their generated files all define an identical, idempotent setup function.
func SetupFunctionName(cfg cache.Config) string {
	return "setup-" + filepath.Base(cfg.ScriptDir)
}

// setupScriptsCommands returns a single shell.exec command that writes all
// files from scriptsFS into scriptDir on the Evergreen agent.
func setupScriptsCommands(
	scriptsFS fs.FS,
	scriptDir string,
) ([]*shrub.CommandDefinition, error) {
	// Use TrimRight to normalize the dir the same way scriptPath() does.
	prefix := strings.TrimRight(scriptDir, "/")

	sentinel := prefix + "/.setup-complete"

	var sb strings.Builder
	sb.WriteString("set -o errexit\nset -o nounset\nset -o pipefail\n")
	// Skip setup if a previous run already wrote the scripts successfully.
	// This makes the function safe to define in multiple included YAML files
	// and safe to call multiple times within a task group.
	fmt.Fprintf(&sb, "if [ -f %q ]; then exit 0; fi\n", sentinel)
	fmt.Fprintf(&sb, "mkdir -p %q\n", prefix)

	err := fs.WalkDir(scriptsFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		return appendScriptHeredoc(&sb, scriptsFS, path, prefix)
	})
	if err != nil {
		return nil, err
	}
	// Write the sentinel last so an interrupted run does not skip a partial setup.
	fmt.Fprintf(&sb, "touch %q\n", sentinel)

	return []*shrub.CommandDefinition{
		shrub.CmdExecShell{
			Script: sb.String(),
		}.Resolve(),
	}, nil
}

// appendScriptHeredoc writes shell commands to sb that create a single script
// file at prefix/<basename> using a heredoc, then make it executable if it is
// a shell script.
func appendScriptHeredoc(sb *strings.Builder, scriptsFS fs.FS, path, prefix string) error {
	content, err := fs.ReadFile(scriptsFS, path)
	if err != nil {
		return err
	}

	scriptName := filepath.Base(path)
	destPath := prefix + "/" + scriptName

	// Build a per-file heredoc delimiter from the full path (not just the
	// basename) so that two files with the same name in different subdirectories
	// produce distinct delimiters.
	delimSuffix := strings.Map(func(r rune) rune {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return '_'
	}, path)
	delimiter := "EVGCACHE_EOF_" + strings.ToUpper(delimSuffix)

	fmt.Fprintf(sb, "cat > %q << '%s'\n", destPath, delimiter)
	sb.Write(content)
	// Ensure heredoc body ends with a newline before the closing delimiter.
	if !bytes.HasSuffix(content, []byte("\n")) {
		sb.WriteByte('\n')
	}
	sb.WriteString(delimiter + "\n")

	if strings.HasSuffix(scriptName, ".sh") {
		fmt.Fprintf(sb, "chmod +x %q\n", destPath)
	}

	return nil
}
