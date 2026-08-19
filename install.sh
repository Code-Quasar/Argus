#!/usr/bin/env bash

set -euo pipefail

project_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
install_dir="${HOME}/.local/bin"
binary_path="${install_dir}/Argus"
profile_path="${HOME}/.profile"
path_line='export PATH="$HOME/.local/bin:$PATH"'

if ! command -v go >/dev/null 2>&1; then
    printf 'error: Go is not installed or is not available in PATH\n' >&2
    exit 1
fi

mkdir -p "$install_dir"

tmp_binary=$(mktemp "${TMPDIR:-/tmp}/Argus.XXXXXX")
trap 'rm -f "$tmp_binary"' EXIT

printf 'Building Argus...\n'
cd "$project_dir"
go build -o "$tmp_binary" .

install -m 755 "$tmp_binary" "$binary_path"

if ! grep -Fqx "$path_line" "$profile_path" 2>/dev/null; then
    printf '\n%s\n' "$path_line" >> "$profile_path"
fi

printf 'Installed: %s\n' "$binary_path"
printf '\nTo use Argus in this terminal, run:\n'
printf '  export PATH="$HOME/.local/bin:$PATH"\n'
printf '\nThen verify with:\n'
printf '  Argus --help\n'
