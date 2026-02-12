# migration-tools

A library of reusable components for use in MongoDB migration tooling.
You may find some of them useful in your own projects.

**IMPORTANT:** This Repository is **NOT** an officially supported MongoDB
product. No support guarantees are made. Use at your own risk.

## Guidelines for contributing

- Prioritize separation of concerns. In particular, avoid references to
specific downstream tools except to give examples.
- Document thoroughly, preferably with example code.

## Do not add …

Some things should _not_ be in this library because upstream tooling already
provides them.

- generic `syncmap`; use [xsync](github.com/puzpuzpuz/xsync/v4) instead
- sets; use [golang-set](github.com/deckarep/golang-set/v2) instead
