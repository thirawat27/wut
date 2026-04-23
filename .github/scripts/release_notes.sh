#!/usr/bin/env bash
set -euo pipefail

changelog_path="${1:-CHANGELOG.md}"
release_tag="${2:-}"
output_path="${3:-release-notes.md}"

release_version="${release_tag#v}"
latest_section=""
matching_section=""
current_version=""
current_section=""

flush_section() {
  if [[ -z "${current_version}" ]]; then
    return
  fi

  if [[ -z "${latest_section}" ]]; then
    latest_section="${current_section}"
  fi

  if [[ "${current_version}" == "${release_version}" ]]; then
    matching_section="${current_section}"
  fi
}

trim_section() {
  awk '
    { lines[++count] = $0 }
    END {
      while (count > 0 && lines[count] ~ /^[[:space:]]*$/) {
        count--
      }

      if (count > 0 && lines[count] ~ /^---[[:space:]]*$/) {
        count--
        while (count > 0 && lines[count] ~ /^[[:space:]]*$/) {
          count--
        }
      }

      for (i = 1; i <= count; i++) {
        print lines[i]
      }
    }
  '
}

if [[ ! -f "${changelog_path}" ]]; then
  {
    echo "Release ${release_tag}"
    echo
    echo "- Cross-platform portable bundles for Linux, macOS, and Windows"
    echo "- Includes the WUT binary, README, and install scripts"
    echo
    echo "No CHANGELOG.md was found in this repository, so these notes were generated automatically."
  } > "${output_path}"
  exit 0
fi

while IFS= read -r line || [[ -n "${line}" ]]; do
  if [[ "${line}" =~ ^##[[:space:]]\[(.+)\] ]]; then
    flush_section
    current_version="${BASH_REMATCH[1]}"
    current_section="${line}"$'\n'
    continue
  fi

  if [[ "${line}" =~ ^##[[:space:]] && -n "${current_version}" ]]; then
    flush_section
    current_version=""
    current_section=""
    continue
  fi

  if [[ -n "${current_version}" ]]; then
    current_section+="${line}"$'\n'
  fi
done < "${changelog_path}"

flush_section

selected_section="${matching_section}"
if [[ -z "${selected_section}" ]]; then
  selected_section="${latest_section}"
fi

if [[ -z "${selected_section}" ]]; then
  {
    echo "Release ${release_tag}"
    echo
    echo "No version sections were found in ${changelog_path}."
  } > "${output_path}"
  exit 0
fi

printf '%s' "${selected_section}" | trim_section > "${output_path}"
