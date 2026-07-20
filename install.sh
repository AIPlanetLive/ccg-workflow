#!/usr/bin/env bash
set -euo pipefail

repo_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
source_dir="${repo_dir}/codeagent-wrapper"
install_dir="${HOME:?HOME is not set}/.claude/bin"
install_path="${install_dir}/codeagent-wrapper"
staged_path=""

cleanup() {
  if [[ -n "${staged_path}" ]]; then
    rm -f "${staged_path}"
  fi
}
trap cleanup EXIT

mkdir -p "${install_dir}"
if [[ -d "${install_path}" ]]; then
  echo "Cannot install codeagent-wrapper: ${install_path} is a directory" >&2
  exit 1
fi

staged_path="$(mktemp "${install_dir}/.codeagent-wrapper.XXXXXX")"

echo "Building codeagent-wrapper..."
(
  cd "${source_dir}"
  go build -o "${staged_path}" .
)

chmod 755 "${staged_path}"
go run "${repo_dir}/scripts/atomic-replace.go" "${staged_path}" "${install_path}"
staged_path=""

echo "Installed codeagent-wrapper to ${install_path}"
