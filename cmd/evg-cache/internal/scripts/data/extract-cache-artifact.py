#!/usr/bin/env python3
# This script expects to be called with the working directory set to ${workdir}.
import argparse
import pathlib
import subprocess


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--artifact", required=True, metavar="PATH")
    args = parser.parse_args()

    p = pathlib.Path(args.artifact)

    if not p.exists():
        print(f"The artifact '{args.artifact}' does not exist. Exiting.")
        return

    # A 0-size file is created by Evergreen when an optional s3.get finds nothing.
    # See https://jira.mongodb.org/browse/DEVPROD-17632
    if p.stat().st_size == 0:
        print(f"The artifact '{args.artifact}' exists but has a 0 size. Exiting.")
        return

    print(f"Extracting '{args.artifact}' in '{pathlib.Path.cwd()}'")
    subprocess.run(["tar", "--extract", "--file", args.artifact, "--xz"], check=True)


if __name__ == "__main__":
    main()
