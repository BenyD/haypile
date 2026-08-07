#!/usr/bin/env sh
# Builds one MCP Bundle (.mcpb) per platform from the goreleaser dist
# output. An .mcpb is a zip with a manifest.json that MCP clients (and
# the official registry, and Smithery) understand; the manifest's
# platform_overrides only split by OS, not architecture, so each
# platform/arch gets its own bundle, mirroring the release archives.
#
#   goreleaser release   # or: goreleaser build (populates dist/)
#   ./hack/build-mcpb.sh v0.3.0
set -eu

version="${1:?usage: build-mcpb.sh vX.Y.Z}"
bare="${version#v}"
root="$(cd "$(dirname "$0")/.." && pwd)"
dist="$root/dist"
out="$dist/mcpb"
mkdir -p "$out"

build() {
  goos="$1" goarch="$2" srcdir="$3" binname="$4"
  stage="$out/stage_${goos}_${goarch}"
  rm -rf "$stage"
  mkdir -p "$stage/server"
  cp "$dist/$srcdir/$binname" "$stage/server/$binname"
  chmod +x "$stage/server/$binname"

  platform="$goos"
  [ "$goos" = "windows" ] && platform="win32"

  cat > "$stage/manifest.json" <<EOF
{
  "manifest_version": "0.3",
  "name": "haypile",
  "display_name": "Haypile",
  "version": "$bare",
  "description": "Private document search, for you and your agents. Hybrid search over local folders with file and page citations; the index never leaves the machine.",
  "author": { "name": "Beny Dishon K", "url": "https://github.com/BenyD" },
  "homepage": "https://haypile.sh",
  "repository": { "type": "git", "url": "https://github.com/BenyD/haypile" },
  "license": "AGPL-3.0-only",
  "server": {
    "type": "binary",
    "entry_point": "server/$binname",
    "mcp_config": {
      "command": "\${__dirname}/server/$binname",
      "args": ["mcp-stdio"]
    }
  },
  "compatibility": { "platforms": ["$platform"] }
}
EOF

  bundle="$out/haypile_${bare}_${goos}_${goarch}.mcpb"
  rm -f "$bundle"
  (cd "$stage" && zip -qr "$bundle" manifest.json server)
  rm -rf "$stage"
  echo "built $(basename "$bundle") ($(wc -c < "$bundle" | tr -d ' ') bytes)"
}

build darwin  arm64 hay_darwin_arm64_v8.0 hay
build darwin  amd64 hay_darwin_amd64_v1   hay
build linux   arm64 hay_linux_arm64_v8.0  hay
build linux   amd64 hay_linux_amd64_v1    hay
build windows amd64 hay_windows_amd64_v1  hay.exe

(cd "$out" && shasum -a 256 ./*.mcpb > mcpb-checksums.txt && cat mcpb-checksums.txt)
