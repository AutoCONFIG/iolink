#!/bin/sh
set -e

src=web/dist
dst=internal/web/dist

rm -rf "$dst"
mkdir -p "$dst"

if [ ! -f "$src/index.html" ]; then
    echo "frontend build missing: $src/index.html (run make web-verify first)" >&2
    exit 1
fi

cp -r "$src"/. "$dst/"
