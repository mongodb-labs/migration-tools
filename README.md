# migration-tools

A library of reusable components for use in MongoDB migration tooling. You may find some of them
useful in your own projects.

**IMPORTANT:** This Repository is **NOT** an officially supported MongoDB product. No support
guarantees are made. Use at your own risk.

## Guidelines for contributing

- Prioritize separation of concerns. In particular, avoid references to specific downstream tools
  except to give examples.
- Document thoroughly, preferably with example code.

### Tests

Unit tests are written in the usual Go way.

Integration tests MUST include the word `Integration` in the test’s name. Call
`internal.GetConnStr()` to fetch the connection string for use in your tests.

## Installing dev tools

This repo uses [`mise`](https://mise.jdx.dev/) for managing dev tools. After you install `mise`, run
the following commands:

```
mise trust
mise install
```

### Pre-commit hooks

If you'd like linting checks to run as a git pre-commit hook, run `git/setup` to install the hook.

## Do not add …

Some things should _not_ be in this library because upstream tooling already provides them.

- generic `syncmap`; use [xsync](https://pkg.go.dev/github.com/puzpuzpuz/xsync/v4) instead
- safe numeric conversion; use
  [github.com/ccoveille/go-safecast/v2](https://pkg.go.dev/github.com/ccoveille/go-safecast/v2)
  instead
- sets; use [golang-set](https://pkg.go.dev/github.com/deckarep/golang-set/v2) instead
