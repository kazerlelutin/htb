#!/bin/sh
# Install the HTB CLI for the current user. Supports Linux x86_64 releases.
set -eu

repository="${HTB_REPOSITORY:-kazerlelutin/htb}"
version="${HTB_VERSION:-latest}"
install_dir="${HTB_INSTALL_DIR:-$HOME/.local/bin}"

if [ "$(uname -s)" != "Linux" ] || [ "$(uname -m)" != "x86_64" ]; then
	printf '%s\n' "HTB installer currently supports Linux x86_64 only." >&2
	exit 1
fi

if [ "$version" = "latest" ]; then
	base_url="https://github.com/$repository/releases/latest/download"
else
	base_url="https://github.com/$repository/releases/download/$version"
fi
archive="htb_linux_amd64.tar.gz"
temporary_dir="$(mktemp -d)"
trap 'rm -rf "$temporary_dir"' EXIT HUP INT TERM

download() {
	if command -v curl >/dev/null 2>&1; then
		curl -fsSL "$1" -o "$2"
		return
	fi
	if command -v wget >/dev/null 2>&1; then
		wget -qO "$2" "$1"
		return
	fi
	printf '%s\n' "curl or wget is required to install HTB." >&2
	exit 1
}

download "$base_url/$archive" "$temporary_dir/$archive"
download "$base_url/checksums.txt" "$temporary_dir/checksums.txt"
if ! (cd "$temporary_dir" && grep " $archive$" checksums.txt | sha256sum -c -); then
	printf '%s\n' "Checksum verification failed; HTB was not installed." >&2
	exit 1
fi
tar -xzf "$temporary_dir/$archive" -C "$temporary_dir" htb
mkdir -p "$install_dir"
install -m 0755 "$temporary_dir/htb" "$install_dir/htb"

case ":$PATH:" in
	*":$install_dir:"*) path_ready=true ;;
	*) path_ready=false ;;
esac
if [ "$path_ready" = false ] && [ "$install_dir" = "$HOME/.local/bin" ]; then
	case "${SHELL:-}" in
	*/zsh) profile="$HOME/.zshrc" ;;
	*) profile="$HOME/.bashrc" ;;
	esac
	path_line='export PATH="$HOME/.local/bin:$PATH"'
	if ! grep -Fqx "$path_line" "$profile" 2>/dev/null; then
		printf '\n# HTB CLI\n%s\n' "$path_line" >> "$profile"
	fi
	printf 'HTB installé dans %s. Ouvrez un nouveau terminal pour utiliser htb.\n' "$install_dir"
else
	printf 'HTB installé : %s/htb\n' "$install_dir"
fi
"$install_dir/htb" version
