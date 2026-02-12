# migration-tools

A library of reusable components for use in MongoDB migration tooling.
You may find some of them useful in your own projects.

**IMPORTANT:** This Repository is **NOT** an officially supported MongoDB
product. No support guarantees are made. Use at your own risk.

## Organization

When updating this library, prioritize separation of concerns.

In particular:
- Avoid references to specific downstream tools except to give examples.

## Do not add …

Some things should _not_ be in this library because upstream tooling already
provides them.

- a generic `syncmap`; use [xsync](github.com/puzpuzpuz/xsync/v4) instead
- sets; use [golang-set](github.com/deckarep/golang-set/v2) instead
