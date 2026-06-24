#!/usr/bin/env python3
"""Compare two JSON schema files using normalized (key-sorted) comparison.

Usage:
    python3 compare_schema.py <live.json> <pinned.json>

Exits 0 if schemas are semantically identical, 1 if they differ.
"""
import json
import sys


def main():
    if len(sys.argv) != 3:
        print(f"Usage: {sys.argv[0]} <live.json> <pinned.json>", file=sys.stderr)
        sys.exit(2)

    live_path, pinned_path = sys.argv[1], sys.argv[2]

    try:
        with open(live_path) as f:
            live = json.load(f)
    except Exception as e:
        print(f"error reading {live_path}: {e}", file=sys.stderr)
        sys.exit(2)

    try:
        with open(pinned_path) as f:
            pinned = json.load(f)
    except Exception as e:
        print(f"error reading {pinned_path}: {e}", file=sys.stderr)
        sys.exit(2)

    live_norm = json.dumps(live, sort_keys=True)
    pinned_norm = json.dumps(pinned, sort_keys=True)

    if live_norm == pinned_norm:
        print("Schema OK: no drift detected")
        sys.exit(0)
    else:
        print("SCHEMA DRIFT DETECTED — run: make generate", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
