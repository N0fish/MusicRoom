#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR=$(cd -- "$(dirname "$0")" && pwd)
REPO_ROOT=$(cd -- "$SCRIPT_DIR/.." && pwd)

if ! command -v swiftlint >/dev/null 2>&1; then
  echo "warning: SwiftLint not installed. Install via Homebrew (brew install swiftlint)" >&2
else
  (cd "$REPO_ROOT" && swiftlint --quiet --config mobile/.swiftlint.yml)
fi

if ! command -v swiftformat >/dev/null 2>&1; then
  echo "warning: SwiftFormat not installed. Install via Homebrew (brew install swiftformat)" >&2
else
  (cd "$REPO_ROOT" && swiftformat --config mobile/.swiftformat --lint mobile/Targets mobile/Project.swift)
fi
