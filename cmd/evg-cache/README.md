# evg-cache

`evg-cache` generates Evergreen CI YAML for S3-backed build artifact caching. It produces
`functions:` blocks that any Evergreen project can include to restore and save a cache keyed by a
hash of configurable files.

## How it works

Cache keys are computed **at CI runtime** by hashing a list of files you specify. This means the
Evergreen config does not need to be regenerated when cache inputs change — only the files
themselves need updating.

The generated YAML defines three Evergreen functions:

- `setup-<script-dir>` — writes the runtime Python and shell scripts to `--script-prefix` on the
  agent; the name is derived from the last path component of `--script-prefix` (default:
  `setup-evg-cache-scripts`). The `generate` command only emits this function when the output file
  does not already define it, so regenerating an existing file never produces a duplicate. The
  function is also idempotent at runtime: it writes the scripts once and marks completion with a
  sentinel file, so repeated calls within a task group are instant no-ops.
- `restore-<name>-cache` — downloads and extracts the cached artifact from S3; sets a
  `<name>_cache_hit` expansion to `true` or `false`
- `save-<name>-cache` — creates a tarball of configured paths (skipped on a cache hit) and uploads
  it to S3, skipping the upload if the object already exists

## Setup

### 1. Generate the functions YAML

```sh
evg-cache generate \
  --name <cache-name> \
  --bucket <s3-bucket> \
  --namespace <s3-namespace> \
  --key-file <path/to/file1> \
  --key-file <path/to/file2> \
  --cache-path <path/to/cache/dir1> \
  --cache-path <path/to/cache/dir2> \
  --output-file evergreen/functions/<cache-name>.yml
git add evergreen/functions/<cache-name>.yml
git commit -m "chore: generate evg-cache YAML for <cache-name>"
```

Regenerate and recommit whenever cache inputs or paths change.

### 2. Wire into your Evergreen config

In `evergreen.yml`, add `setup-evg-cache-scripts` to `pre:` so it runs automatically before every
task — no per-task setup calls needed:

```yaml
include:
  - filename: evergreen/functions/<cache-name>.yml

pre:
  - func: setup-evg-cache-scripts

tasks:
  - name: my-task
    commands:
      - func: restore-<name>-cache
      - command: subprocess.exec # your install step
        params:
          binary: ./scripts/install-deps.sh
          include_expansions_in_env: [<name>_cache_hit, workdir]
      - func: save-<name>-cache
```

If you have multiple caches, include all of their generated files. Each new file gets
`setup-evg-cache-scripts` on first generation; regenerating an existing file that already has it
will omit the duplicate. If multiple included files end up defining the function, the runtime
sentinel ensures only the first execution does any work.

## generate flags

| Flag              | Required       | Default               | Description                                      |
| ----------------- | -------------- | --------------------- | ------------------------------------------------ |
| `--name`          | ✓              |                       | Cache name; must match `[a-zA-Z0-9-]+`           |
| `--bucket`        | ✓              |                       | S3 bucket                                        |
| `--namespace`     | ✓              |                       | S3 path prefix (e.g. `myproject/go-cache`)       |
| `--key-file`      | ✓ (repeatable) |                       | File to hash for the cache key                   |
| `--cache-path`    | ✓ (repeatable) |                       | Path to bundle into the artifact tarball         |
| `--display-name`  |                | `--name` value        | Human-readable label for the S3 upload           |
| `--script-prefix` |                | `./evg-cache-scripts` | Directory to write runtime scripts into on agent |
| `--output-file`   |                | stdout                | Write output to this file instead of stdout      |

## extract-scripts command

The `extract-scripts` subcommand writes the runtime scripts to a local directory. This is an
alternative to the generated `setup-evg-cache-scripts` function for teams that prefer to commit the
scripts to their repo rather than generating them at CI time.

| Flag           | Required | Description                                 |
| -------------- | -------- | ------------------------------------------- |
| `--output-dir` | ✓        | Directory to write the runtime scripts into |

## Example: mongosync mise-and-go cache

```sh
evg-cache generate \
  --name mise-and-go \
  --bucket mciuploads \
  --namespace mongosync/mise-cache \
  --key-file src/mongosync/mise.toml \
  --key-file src/mongosync/magefiles/buildlib/mise-version.txt \
  --cache-path .local/bin/mise \
  --cache-path .local/share/mise \
  --display-name "mise and go executables" \
  --output-file evergreen/functions/mise-and-go-cache.yml
```
