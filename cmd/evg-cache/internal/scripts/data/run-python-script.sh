#!/bin/bash
set -o errexit
set -o pipefail

SCRIPT_DIR=$(dirname "$0")
# shellcheck source=cmd/evg-cache/internal/scripts/data/find-recent-python.sh
. "$SCRIPT_DIR/find-recent-python.sh"
exec python3 "$@"
