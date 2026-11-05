#!/usr/bin/env bash
# Assembles the GitHub Pages site into _site/ (or the directory given).
# Run from the repository root: site/build.sh [outdir]
# The recording is converted from demo/demo.gif so the README GIF stays the
# one source; without ffmpeg the page falls back to the GIF itself.
set -euo pipefail
out=${1:-_site}
rm -rf "$out"
mkdir -p "$out"
cp site/index.html site/style.css site/app.js site/favicon.svg site/og.png \
   site/404.html site/robots.txt site/sitemap.xml "$out"/
cp demo/demo.gif "$out"/demo.gif

# Latest release tag, when tags are available (CI fetches them).
if tag=$(git describe --tags --abbrev=0 2>/dev/null); then
  sed -i.bak "s|<!--version-->[^<]*<!--/version-->|$tag|" "$out/index.html" && rm -f "$out/index.html.bak"
fi

if command -v ffmpeg >/dev/null; then
  ffmpeg -loglevel error -y -i demo/demo.gif -movflags +faststart -pix_fmt yuv420p \
    -vf "scale=trunc(iw/2)*2:trunc(ih/2)*2" -c:v libx264 -crf 24 -preset slow "$out/demo.mp4"
  ffmpeg -loglevel error -y -i demo/demo.gif -vf "select=eq(n\,100)" -frames:v 1 -c:v libwebp -quality 82 "$out/demo-poster.webp" || true
else
  echo "ffmpeg not found: the page will show demo.gif instead of a video" >&2
fi
ls -la "$out"
