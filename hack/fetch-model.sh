#!/bin/sh
# Fetches the bundled embedding model into the source tree so release
# builds (`go build -tags bundled`) can embed it. The file is gitignored.
set -eu

root="$(dirname "$0")/.."
dir="$root/internal/embed/bundled"
url="https://huggingface.co/sentence-transformers/all-MiniLM-L6-v2/resolve/main/model.safetensors"

if [ ! -f "$dir/model.safetensors" ]; then
    echo "fetching all-MiniLM-L6-v2 weights (~90MB)..."
    curl -fSL -o "$dir/model.safetensors" "$url"
fi

# Release builds embed the int8-quantized weights (~3x smaller binary);
# the loader dequantizes at startup so inference speed is unchanged.
if [ ! -f "$dir/model.q8.safetensors" ]; then
    (cd "$root" && go run ./internal/embed/bundled/quantize)
fi
echo "done: $dir/model.safetensors + $dir/model.q8.safetensors"
