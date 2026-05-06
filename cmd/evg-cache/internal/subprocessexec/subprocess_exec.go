// cmd/evg-cache/internal/subprocessexec/subprocess_exec.go
package subprocessexec

import "github.com/evergreen-ci/shrub"

// CmdBuilder builds subprocess.exec Evergreen commands.
type CmdBuilder struct {
	binary                 string
	args                   []string
	includeExpansionsInEnv []string
	workingDir             string
}

func NewCmdBuilder(binary string) *CmdBuilder {
	return &CmdBuilder{binary: binary}
}

func (b *CmdBuilder) WithWorkingDirectory(dir string) *CmdBuilder {
	b.workingDir = dir
	return b
}

func (b *CmdBuilder) WithArgs(args ...string) *CmdBuilder {
	b.args = args
	return b
}

func (b *CmdBuilder) WithIncludeExpansionsInEnv(expansions ...string) *CmdBuilder {
	b.includeExpansionsInEnv = expansions
	return b
}

func (b *CmdBuilder) Build() *shrub.CommandDefinition {
	return shrub.CmdExec{
		Binary:                 b.binary,
		Args:                   b.args,
		IncludeExpansionsInEnv: b.includeExpansionsInEnv,
		WorkingDirectory:       b.workingDir,
	}.Resolve()
}
