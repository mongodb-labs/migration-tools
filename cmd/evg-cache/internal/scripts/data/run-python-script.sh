#!/bin/bash
set -o errexit
set -o nounset
set -o pipefail
if [ -n "${EVG_CACHE_VERBOSE:-}" ]; then
    set -o verbose
fi

SCRIPT_DIR=$(dirname "$0")
# shellcheck source=cmd/evg-cache/internal/scripts/data/find-recent-python.sh
. "$SCRIPT_DIR/find-recent-python.sh"
exec "$python_path" "$@"
