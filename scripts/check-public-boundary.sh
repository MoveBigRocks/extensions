#!/usr/bin/env bash

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

cd "${repo_root}"

if rg -n 'github\.com/movebigrocks/platform/internal/' --glob '*.go' .; then
  echo >&2
  echo "External extension repos must not import github.com/movebigrocks/platform/internal/..." >&2
  echo "Use public SDK packages or github.com/movebigrocks/platform/pkg/extensionhost/... instead." >&2
  exit 1
fi

echo "Public boundary check passed."
