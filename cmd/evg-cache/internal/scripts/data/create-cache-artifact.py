#!/usr/bin/env python3
# This script expects to be called with the working directory set to ${workdir}.
import argparse
import os
import subprocess


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--artifact", required=True, metavar="PATH")
    parser.add_argument("--path", required=True, action="append", metavar="PATH", dest="paths")
    parser.add_argument("--cache-hit-expansion", required=True, metavar="NAME")
    args = parser.parse_args()

    if os.environ.get(args.cache_hit_expansion) == "true":
        print(f"Cache hit ({args.cache_hit_expansion}=true): skipping tarball creation")
        return

    print(f"Cache miss: creating '{args.artifact}' from {args.paths}")
    subprocess.run(
        ["tar", "--create", "--xz", "--file", args.artifact, "--"] + args.paths,
        check=True,
    )


if __name__ == "__main__":
    main()
