#!/bin/bash

set -o errexit
set -o pipefail
set -o verbose

WORKDIR="$1"
distro_id=""

if [ -f /etc/os-release ]; then
    . /etc/os-release
    distro_id="$ID-$VERSION_ID-$(uname -p)"
elif [ "$(uname)" = "Darwin" ]; then
    macos_version="$(sw_vers -productVersion | cut -d. -f1)"
    distro_id="macos-${macos_version}-$(uname -p)"
else
    echo "Could not determine distro id for this distro!"
    exit 1
fi

echo "Setting distro_id to '$distro_id'"

echo "distro_id: $distro_id" >"$WORKDIR/distro-id.yml"
