#!/usr/bin/env bash

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

cd "${repo_root}"

if rg -n 'github\.com/movebigrocks/platform/' --glob '*.go' .; then
  echo >&2
  echo "Extensions must not import github.com/movebigrocks/platform/..." >&2
  echo "Use github.com/movebigrocks/extension-sdk/... or local extension packages instead." >&2
  exit 1
fi

echo "Public boundary check passed."
