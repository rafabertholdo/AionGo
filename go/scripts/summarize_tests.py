#!/usr/bin/env python3
"""Summarize go test -json output, including unfinished tests and packages."""

import argparse
from collections import Counter
import json
from pathlib import Path


def summarize(lines):
    tests = Counter()
    packages = Counter()
    active_tests = set()
    active_packages = set()
    failures = []
    skipped = []
    events = 0
    for line in lines:
        if not line.strip():
            continue
        event = json.loads(line)
        events += 1
        action = event.get("Action")
        package = event.get("Package", "")
        test = event.get("Test")
        if test:
            name = f"{package}/{test}"
            if action == "run":
                active_tests.add(name)
            elif action in ("pass", "fail", "skip"):
                active_tests.discard(name)
                tests[action] += 1
                if action == "fail":
                    failures.append(name)
                elif action == "skip":
                    skipped.append(name)
        elif action == "start":
            active_packages.add(package)
        elif action in ("pass", "fail", "skip"):
            active_packages.discard(package)
            packages[action] += 1
            if action == "fail":
                failures.append(package)
    return {
        "events": events,
        "test_nodes": {key: tests[key] for key in ("pass", "fail", "skip")},
        "packages": {key: packages[key] for key in ("pass", "fail", "skip")},
        "failures": failures,
        "skipped_tests": skipped,
        "unfinished_tests": sorted(active_tests),
        "unfinished_packages": sorted(active_packages),
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("log", type=Path)
    args = parser.parse_args()
    try:
        with args.log.open() as stream:
            report = summarize(stream)
    except (OSError, ValueError) as error:
        parser.exit(2, f"cannot summarize test output: {error}\n")
    print(json.dumps(report, indent=2))
    return int(
        not report["events"]
        or not sum(report["packages"].values())
        or bool(report["failures"])
        or bool(report["unfinished_tests"])
        or bool(report["unfinished_packages"])
    )


if __name__ == "__main__":
    raise SystemExit(main())
