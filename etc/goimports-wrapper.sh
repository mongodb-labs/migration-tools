#!/usr/bin/env bash

set -o errexit

if [ "$1" == "--lint" ]; then
    OUTPUT=$(mise exec 'go:golang.org/x/tools/cmd/goimports' -- goimports -d -e "${@:2}")
    if [ -n "$OUTPUT" ]; then
        echo "$OUTPUT"
        exit 1
    fi
else
    mise exec 'go:golang.org/x/tools/cmd/goimports' -- goimports -w "${@:1}"
fi
