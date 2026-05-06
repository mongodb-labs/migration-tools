#!/usr/bin/env python3
import argparse
import hashlib
import pathlib
import sys


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--file", dest="files", action="append", required=True, metavar="PATH")
    parser.add_argument("--output", required=True, metavar="PATH")
    args = parser.parse_args()

    h = hashlib.sha256()
    for f in args.files:
        p = pathlib.Path(f)
        if not p.is_file():
            print(f"Error: file not found: {f}", file=sys.stderr)
            sys.exit(1)
        h.update(p.read_bytes())

    key = h.hexdigest()
    pathlib.Path(args.output).write_text(f"cache_key: {key}\n")
    print(f"Computed cache key: {key} (written to {args.output})")


if __name__ == "__main__":
    main()
