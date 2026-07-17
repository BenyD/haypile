#!/bin/sh
# Haypile installer: detects your platform, downloads the latest release
# binary from GitHub, and installs it as `hay`.
#
#   curl -fsSL https://raw.githubusercontent.com/BenyD/haypile/main/install.sh | sh
#
# Nothing here phones home. The only network request is the download from
# GitHub Releases, and you are reading the script that makes it.
set -eu

REPO="BenyD/haypile"
INSTALL_DIR="${HAY_INSTALL_DIR:-/usr/local/bin}"

main() {
    os=$(uname -s | tr '[:upper:]' '[:lower:]')
    arch=$(uname -m)
    case "$arch" in
        x86_64) arch="amd64" ;;
        aarch64 | arm64) arch="arm64" ;;
        *) fail "unsupported architecture: $arch" ;;
    esac
    case "$os" in
        darwin | linux) ;;
        *) fail "unsupported OS: $os (Windows: download the zip from https://github.com/$REPO/releases)" ;;
    esac

    version=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" |
        grep '"tag_name"' | head -1 | cut -d '"' -f 4)
    [ -n "$version" ] || fail "could not determine the latest release"

    plain=${version#v}
    url="https://github.com/$REPO/releases/download/$version/haypile_${plain}_${os}_${arch}.tar.gz"

    echo "Installing hay $version for $os/$arch..."
    tmp=$(mktemp -d)
    trap 'rm -rf "$tmp"' EXIT
    curl -fsSL "$url" -o "$tmp/hay.tar.gz"
    tar -xzf "$tmp/hay.tar.gz" -C "$tmp"

    if [ -w "$INSTALL_DIR" ]; then
        mv "$tmp/hay" "$INSTALL_DIR/hay"
    else
        echo "Writing to $INSTALL_DIR needs sudo:"
        sudo mv "$tmp/hay" "$INSTALL_DIR/hay"
    fi

    echo "Installed: $INSTALL_DIR/hay"
    echo
    echo "Get started:"
    echo '  hay add ~/Documents'
    echo '  hay search "something you remember"'
}

fail() {
    echo "install.sh: $1" >&2
    exit 1
}

main
