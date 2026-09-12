#!/usr/bin/env python3
"""Print the authoritative verdict of the newest run.json under a dir.

run.json carries the CaseResult verdict written through by the harness
(release review finding #1): this is the same value result.json and the
signed bundle agree on. Exits non-zero when no run record is found.
"""
import json
import os
import sys


def main() -> int:
    root = sys.argv[1]
    best_path, best_mtime = None, -1.0
    for dirpath, _, files in os.walk(root):
        if "run.json" in files:
            path = os.path.join(dirpath, "run.json")
            mtime = os.path.getmtime(path)
            if mtime > best_mtime:
                best_path, best_mtime = path, mtime
    if best_path is None:
        return 1
    with open(best_path, encoding="utf-8") as handle:
        print(json.load(handle).get("verdict", ""))
    return 0


if __name__ == "__main__":
    sys.exit(main())
