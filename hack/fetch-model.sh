#!/bin/sh
# Fetches the bundled embedding model into the source tree so release
# builds (`go build -tags bundled`) can embed it. The file is gitignored.
set -eu

dir="$(dirname "$0")/../internal/embed/bundled"
url="https://huggingface.co/sentence-transformers/all-MiniLM-L6-v2/resolve/main/model.safetensors"

if [ -f "$dir/model.safetensors" ]; then
    echo "model already present: $dir/model.safetensors"
    exit 0
fi

echo "fetching all-MiniLM-L6-v2 weights (~90MB)..."
curl -fSL -o "$dir/model.safetensors" "$url"
echo "done: $dir/model.safetensors"
