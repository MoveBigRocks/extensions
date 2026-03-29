#!/usr/bin/env bash

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
mbr_bin="${MBR_BIN:-mbr}"

if ! command -v "${mbr_bin}" >/dev/null 2>&1; then
  echo "mbr CLI not found: ${mbr_bin}" >&2
  echo "Set MBR_BIN to the built CLI path or add mbr to PATH." >&2
  exit 1
fi

source_dirs=()
while IFS= read -r manifest_path; do
  source_dirs+=("$(basename "$(dirname "${manifest_path}")")")
done < <(find "${repo_root}" -mindepth 2 -maxdepth 2 -name manifest.json | sort)

if [[ "${#source_dirs[@]}" -eq 0 ]]; then
  echo "No first-party extension manifests found in ${repo_root}" >&2
  exit 1
fi

echo "Validating first-party extension contracts from ${repo_root}"

for source_dir in "${source_dirs[@]}"; do
  extension_dir="${repo_root}/${source_dir}"
  echo
  echo "==> ${source_dir}"
  "${mbr_bin}" extensions lint "${extension_dir}" --json
done
