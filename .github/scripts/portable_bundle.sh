#!/usr/bin/env bash
set -euo pipefail

binary_path="${1:-}"
output_dir="${2:-}"

if [[ -z "${binary_path}" || -z "${output_dir}" ]]; then
  echo "usage: $0 <binary-path> <output-dir>" >&2
  exit 1
fi

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
bundle_root="${repo_root}/${output_dir}"
mkdir -p "${bundle_root}"

copy_if_exists() {
  local source_path="$1"
  local destination_path="$2"

  if [[ -e "${source_path}" ]]; then
    mkdir -p "$(dirname "${destination_path}")"
    cp -R "${source_path}" "${destination_path}"
  fi
}

copy_if_exists "${repo_root}/${binary_path}" "${bundle_root}/$(basename "${binary_path}")"
copy_if_exists "${repo_root}/LICENSE" "${bundle_root}/LICENSE"
copy_if_exists "${repo_root}/README.md" "${bundle_root}/README.md"
copy_if_exists "${repo_root}/CONTRIBUTING.md" "${bundle_root}/CONTRIBUTING.md"
copy_if_exists "${repo_root}/scripts/install.sh" "${bundle_root}/scripts/install.sh"
copy_if_exists "${repo_root}/scripts/install.ps1" "${bundle_root}/scripts/install.ps1"
