#!/usr/bin/env sh
set -eu

if [ "$#" -ne 1 ]; then
  echo "usage: $0 VERSION" >&2
  exit 2
fi

version=$1

printf '# HTB %s\n\n' "$version"
if ! awk -v version="$version" '
  $0 ~ "^## \\[" version "\\]" { found = 1; next }
  found && /^## / { exit }
  found { print }
  END { if (!found) exit 1 }
' CHANGELOG.md; then
  echo "release notes for $version are missing from CHANGELOG.md" >&2
  exit 1
fi

cat <<'EOF'

## Install

- Linux x86_64: `curl -fsSL https://github.com/kazerlelutin/htb/releases/latest/download/install.sh | sh`
- macOS x86_64 or Apple Silicon: download the matching archive from this release.
- Windows x86_64: `irm https://github.com/kazerlelutin/htb/releases/latest/download/install.ps1 | iex`

Each archive contains `htb` and `htbd`; verify it with `checksums.txt` before
manual installation.

## Complete changelog

[Read the complete changelog](https://github.com/kazerlelutin/htb/blob/main/CHANGELOG.md).
EOF
