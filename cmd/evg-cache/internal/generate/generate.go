// Package generate writes Evergreen functions YAML for evg-cache configurations.
package generate

import (
	"io"

	"github.com/evergreen-ci/shrub"
	"github.com/goccy/go-yaml"
	"github.com/mongodb-labs/migration-tools/cmd/evg-cache/internal/cache"
)

// GenerateFunctions writes two Evergreen function definitions to w:
//   - restore-<name>-cache: compute key, download artifact, detect hit, extract
//   - save-<name>-cache: create tarball (on miss), upload to S3
func GenerateFunctions(cfg cache.Config, displayName string, w io.Writer) error {
	restoreName := "restore-" + cfg.Name + "-cache"
	saveName := "save-" + cfg.Name + "-cache"

	restoreSeq := shrub.CommandSequence(
		append(cfg.ComputeKeyCommands(), cfg.RestoreCommands()...),
	)
	saveSeq := shrub.CommandSequence(cfg.SaveCommands(displayName))

	out := struct {
		Functions map[string]*shrub.CommandSequence `yaml:"functions"`
	}{
		Functions: map[string]*shrub.CommandSequence{
			restoreName: &restoreSeq,
			saveName:    &saveSeq,
		},
	}

	enc := yaml.NewEncoder(w)
	if err := enc.Encode(out); err != nil {
		return err
	}
	return enc.Close()
}
