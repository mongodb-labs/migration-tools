# migration-tools

A library of reusable components for MongoDB migration tooling.

## Organization

When augmenting this library, prioritize separation of concerns.

In particular:
- Put MongoDB-specific logic under `mongo/`.
- Avoid references to specific migration tools.
