#!/usr/bin/env python3
# This script expects to be called with the working directory set to ${workdir}.
import argparse
import pathlib
import subprocess


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--artifact", required=True, metavar="PATH")
    parser.add_argument("--output", required=True, metavar="PATH")
    parser.add_argument("--expansion-name", required=True, metavar="NAME")
    args = parser.parse_args()

    p = pathlib.Path(args.artifact)
    # A 0-size file is created by Evergreen when an optional s3.get finds nothing.
    # See https://jira.mongodb.org/browse/DEVPROD-17632
    hit = p.is_file() and p.stat().st_size > 0

    value = "true" if hit else ""
    pathlib.Path(args.output).write_text(f'{args.expansion_name}: "{value}"\n')

    if hit:
        print(f"Cache hit: extracting '{args.artifact}' in '{pathlib.Path.cwd()}'")
        subprocess.run(["tar", "--extract", "--file", args.artifact, "--xz"], check=True)
    else:
        print(f"Cache miss: {args.artifact} does not exist or is empty")


if __name__ == "__main__":
    main()
