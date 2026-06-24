#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
sdk_root="$(cd "${script_dir}/.." && pwd)"
version_file="${sdk_root}/codex-go/VERSION"

if [[ ! -f "${version_file}" ]]; then
  echo "missing version file: ${version_file}" >&2
  exit 1
fi

version="$(tr -d '[:space:]' < "${version_file}")"
if [[ -z "${version}" ]]; then
  echo "empty version file: ${version_file}" >&2
  exit 1
fi

tag="codex-go/${version}"
if git rev-parse "${tag}" >/dev/null 2>&1; then
  echo "tag already exists: ${tag}" >&2
  exit 1
fi

git tag -a "${tag}" -m "codex-go ${version}"
echo "${tag}"
