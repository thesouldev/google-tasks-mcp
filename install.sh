#!/bin/sh
set -eu

repo="thesouldev/google-tasks-mcp"
bin="google-tasks-mcp"
install_dir="${GOOGLE_TASKS_MCP_INSTALL_DIR:-/usr/local/bin}"
version="${GOOGLE_TASKS_MCP_VERSION:-}"

fail() {
	echo "$1" >&2
	exit 1
}

os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$os" in
linux | darwin) ;;
*) fail "Unsupported OS: $os. Download a release from https://github.com/$repo/releases" ;;
esac

case "$(uname -m)" in
x86_64 | amd64) arch=amd64 ;;
aarch64 | arm64) arch=arm64 ;;
*) fail "Unsupported architecture: $(uname -m). Download a release from https://github.com/$repo/releases" ;;
esac

if [ -n "$version" ]; then
	base="https://github.com/$repo/releases/download/v${version#v}"
else
	base="https://github.com/$repo/releases/latest/download"
fi
archive="${bin}_${os}_${arch}.tar.gz"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

echo "Downloading $archive" >&2
curl -fsSL "$base/$archive" -o "$tmp/$archive"
curl -fsSL "$base/checksums.txt" -o "$tmp/checksums.txt"

# A tampered or truncated download must never reach the PATH.
expected=$(grep " $archive\$" "$tmp/checksums.txt" | cut -d' ' -f1)
if command -v sha256sum >/dev/null 2>&1; then
	actual=$(sha256sum "$tmp/$archive" | cut -d' ' -f1)
else
	actual=$(shasum -a 256 "$tmp/$archive" | cut -d' ' -f1)
fi
[ -n "$expected" ] && [ "$expected" = "$actual" ] || fail "Checksum mismatch for $archive, nothing was installed."

tar -xzf "$tmp/$archive" -C "$tmp" "$bin"

mkdir -p "$install_dir" 2>/dev/null || true
if [ -w "$install_dir" ]; then
	install -m 0755 "$tmp/$bin" "$install_dir/$bin"
else
	echo "Installing to $install_dir with sudo" >&2
	sudo install -m 0755 "$tmp/$bin" "$install_dir/$bin"
fi

echo "Installed $bin $("$install_dir/$bin" version) to $install_dir/$bin" >&2
