#!/usr/bin/env bash

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
catalog_path="${repo_root}/catalog/public-bundles.json"
mbr_bin="${MBR_BIN:-mbr}"

if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required to read ${catalog_path}" >&2
  exit 1
fi

if ! command -v "${mbr_bin}" >/dev/null 2>&1; then
  echo "mbr CLI not found: ${mbr_bin}" >&2
  echo "Set MBR_BIN to the built CLI path or add mbr to PATH." >&2
  exit 1
fi

source_dirs=()
while IFS= read -r source_dir; do
  source_dirs+=("${source_dir}")
done < <(jq -r '.bundles[].sourceDir' "${catalog_path}")

if [[ "${#source_dirs[@]}" -eq 0 ]]; then
  echo "No public bundles found in ${catalog_path}" >&2
  exit 1
fi

echo "Validating first-party extension contracts from ${catalog_path}"

for source_dir in "${source_dirs[@]}"; do
  extension_dir="${repo_root}/${source_dir}"
  echo
  echo "==> ${source_dir}"
  "${mbr_bin}" extensions lint "${extension_dir}" --json
done
