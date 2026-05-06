# evg-cache

`evg-cache` generates Evergreen CI YAML for S3-backed build artifact caching. It produces a set of
`functions:` blocks that any Evergreen project can include to restore and save a cache. The cache is
keyed by a hash of configurable files and always includes the host's OS and CPU in the cache path.

This lets you avoid repeatedly doing the same expensive operations, like installing project
dependencies.

## How it works

Cache keys are computed **at CI runtime** by hashing a list of files and (optional) Evergreen
expansions you specify. This means the Evergreen config does not need to be regenerated when cache
inputs change — only the files themselves need updating.

The generated YAML defines three Evergreen functions:

- `set-up-evg-cache-scripts` — writes the runtime Python and shell scripts to `./evg-cache-scripts`
  on the CI machine. The function is idempotent at runtime: it writes the scripts once and marks
  completion with a sentinel file, so if multiple Evergreen tasks call this function, it only writes
  the scripts once.
- `restore-<cache-name>-cache` — downloads and extracts the cached artifact from S3; sets a
  `<cache-name>_cache_hit` expansion to `"true"` on a hit or `""` on a miss.
- `save-<cache-name>-cache` — creates a tarball of configured paths (skipped on a cache hit) and
  uploads it to S3, skipping the upload if the object already exists.

When generate the cache hit expansion name, any dashes in `<cache-name>` will be converted to
underscores.

## Setup

### 1. Install evg-cache

With [mise](https://mise.jdx.dev/):

```toml
# mise.toml
[tools]
"github:mongodb-labs/migration-tools" = { version = "<tag>", bin = "evg-cache" }
```

Or download a release binary from the
[GitHub releases page](https://github.com/mongodb-labs/migration-tools/releases).

### 2. Generate the functions YAML

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

If your project has a config generator that produces Evergreen YAML programmatically, you can call
`evg-cache generate` from it and merge the output into your generated config.

### 3. Wire into your Evergreen config

**Important:** You must call `git.get_project` before `restore-<cache-name>-cache`. The cache key is
computed by hashing files from the checked-out source tree, so those files must exist on the CI
machine first.

#### Option A: `pre` block

Put `set-up-evg-cache-scripts` in Evergreen's `pre:` block so it runs automatically before every
task, then call restore/save around your install step in each task:

```yaml
include:
  - filename: evergreen/functions/<cache-name>.yml

pre:
  - func: set-up-evg-cache-scripts

tasks:
  - name: my-task
    commands:
      - command: git.get_project
        params:
          directory: .
      - func: restore-my-deps-cache
      - command: subprocess.exec # your install step
        params:
          binary: ./scripts/install-deps.sh
          include_expansions_in_env: [my_deps_cache_hit, workdir]
      - func: save-my-deps-cache
      # The actual task work goes here
```

#### Option B: explicit task-level sequencing

If your project doesn't use `pre:` (e.g. because task setup is managed through a config generator),
list all steps explicitly in each task:

```yaml
tasks:
  - name: my-task
    commands:
      - command: git.get_project # checkout source so key files exist
        params:
          directory: .
      - func: set-up-evg-cache-scripts # deploy runtime scripts
      - func: restore-my-deps-cache # download + extract artifact
      - command: subprocess.exec # your install step
        params:
          binary: ./scripts/install-deps.sh
          include_expansions_in_env: [my_deps_cache_hit, workdir]
      - func: save-my-deps-cache # create + upload on cache miss
      # The actual task work goes here
```

This is more verbose but makes the full setup sequence visible in each task and integrates cleanly
with programmatic task generation.

### 4. Update Your Install Scripts

In the examples above, the things being cached are installed by a script named
`./scripts/install-deps.sh`. Let's assume that the cache hit expansion is named `deps_cache_hit`. In
that case, your install script should check whether this is true and not do any work in that case:

```bash
if [ "$deps_cache_hit" = "true" ]; then
    exit 0
fi
```

If your dependencies change, then the next time the script runs, the cache hit expansion will not be
set to `true`, so it will do the install.

## Warming the Cache

If you have many tasks which share a cache, and those tasks run in parallel in Evergreen, then they
will all race to regenerate the cache whenever the contents of the cache need to change. This can be
quite costly. You can prevent this by adding a "cache warming" task that all those tasks depend on.
That task would look exactly like the examples above, except you would not put any commands after
the `save-my-deps-cache` function is called.

```yaml
tasks:
  - name: warm-my-deps-cache
    commands:
      - command: git.get_project
        params:
          directory: .
      - func: restore-my-deps-cache
      - command: subprocess.exec # your install step
        params:
          binary: ./scripts/install-deps.sh
          include_expansions_in_env: [my_deps_cache_hit, workdir]
      - func: save-my-deps-cache
      # No further commands — the only purpose of this task is to warm the cache.

  - name: my-task
    depends_on:
      - name: warm-my-deps-cache
    commands:
      - func: restore-my-deps-cache
      - command: subprocess.exec # your actual task work
        params:
          binary: ./scripts/run-task.sh
          include_expansions_in_env: [my_deps_cache_hit, workdir]
```

Both tasks omit `set-up-evg-cache-scripts` because the `pre:` block (Option A) handles it. If you
are using Option B, add `- func: set-up-evg-cache-scripts` before `restore-my-deps-cache` in both
tasks.

Note that while this is _more_ efficient when the cache is empty, it's _less_ efficient for later CI
runs where the cache is populated. It means that all of the parallel tasks will block on this one
warmup task even though the warmup task doesn't need to do anything because the cache is already
populated.

## Debugging

Set the `EVG_CACHE_VERBOSE` environment variable to any non-empty value to enable verbose shell
output (`set -o verbose`) in all runtime scripts. This prints each shell command before it runs,
which is useful for diagnosing cache misses or S3 failures.

## generate flags

| Flag                    | Required       | Default        | Description                                                                                                                      |
| ----------------------- | -------------- | -------------- | -------------------------------------------------------------------------------------------------------------------------------- |
| `--name`                | ✓              |                | Cache name; must match `[a-zA-Z0-9-]+`                                                                                           |
| `--bucket`              | ✓              |                | S3 bucket                                                                                                                        |
| `--namespace`           | ✓              |                | S3 path prefix (e.g. `myproject/go-cache`)                                                                                       |
| `--key-file`            | ✓ (repeatable) |                | File to hash for the cache key                                                                                                   |
| `--cache-path`          | ✓ (repeatable) |                | Path to bundle into the artifact tarball                                                                                         |
| `--key-expansion`       | (repeatable)   |                | Evergreen expansion value to include in the cache key hash (e.g. `${branch_name}`)                                               |
| `--display-name`        |                | `--name` value | Human-readable label for the S3 upload                                                                                           |
| `--output-file`         |                | stdout         | Write output to this file instead of stdout                                                                                      |
| `--omit-setup-function` |                | false          | Omit `set-up-evg-cache-scripts` from output; use when multiple caches share one setup function defined in another generated file |

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
