# Instructions for AI tooling

After editing files, run `precious tidy <file>` to format them. Do not invoke the underlying tools
(e.g. `goimports`, `golines`, `golangci-lint --fix`) directly — `precious.toml` routes them through
wrapper scripts and `mise exec`, so running them bare will not match the project's required
formatting.

Comply with all linting rules enforced by the tools in `precious.toml`. For example, the staticcheck
rule ST1011 forbids unit-specific suffixes on `time.Duration` variables (e.g. name a `time.Duration`
`remaining`, not `remainingSec`).

Avoid unused parameters. Use `_` to satisfy type definitions where necessary.
