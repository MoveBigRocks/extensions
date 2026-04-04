#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
remote_name="${1:-origin}"
publishable_extensions=(
  ats
  community-feature-requests
  error-tracking
  sales-pipeline
  web-analytics
)

printf "%-30s %-12s %-18s %-8s %-8s\n" "extension" "version" "release_tag" "local" "remote"

while IFS= read -r manifest_path; do
  extension_dir="$(basename "$(dirname "${manifest_path}")")"
  case " ${publishable_extensions[*]} " in
    *" ${extension_dir} "*) ;;
    *) continue ;;
  esac
  version="$(jq -r '.version // ""' "${manifest_path}")"
  if [[ -z "${version}" || "${version}" == "null" ]]; then
    echo "missing version in ${manifest_path}" >&2
    exit 1
  fi

  release_tag="${extension_dir}-v${version}"
  local_state="no"
  remote_state="no"

  if git -C "${repo_root}" rev-parse -q --verify "refs/tags/${release_tag}" >/dev/null; then
    local_state="yes"
  fi

  if git -C "${repo_root}" ls-remote --exit-code --tags "${remote_name}" "refs/tags/${release_tag}" >/dev/null 2>&1; then
    remote_state="yes"
  fi

  printf "%-30s %-12s %-18s %-8s %-8s\n" "${extension_dir}" "${version}" "${release_tag}" "${local_state}" "${remote_state}"
done < <(find "${repo_root}" -mindepth 2 -maxdepth 2 -name manifest.json | sort)
