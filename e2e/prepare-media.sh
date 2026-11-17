#!/bin/sh
# Adapted from jplayer-mobile/e2e/prepare-media.sh at 49de3c2.
#
# Collects the media the device tests play into one folder:
#   e2e/prepare-media.sh <out-dir>
# Everything comes from the mock API's media folder (no real DanceFITVN
# content, CLAUDE.md §4): the lesson l_3503 as HLS (master and 480p
# playlists, three 4 s MPEG-TS segments, 480x854, 12 s) and the sample MP3
# (30 s), laid out as the mock serves them (hls/l_3503/..., audio/...), so
# that the tests serve the folder with internal/api/mock's Server on
# 127.0.0.1. devtest-ios.sh and scripts/tvos-avf-test.sh run it.
set -eu
here=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$here/.." && pwd)
out=${1:?usage: prepare-media.sh <out-dir>}
src="$root/mock/dancefitvn-mock-api/media"
[ -d "$src/hls/l_3503" ] || { echo "prepare-media: no $src/hls/l_3503" >&2; exit 1; }
mkdir -p "$out/hls/l_3503" "$out/audio"
cp "$src"/hls/l_3503/*.m3u8 "$src"/hls/l_3503/*.ts "$out/hls/l_3503/"
cp "$src/audio/_sample.mp3" "$out/audio/_sample.mp3"
echo "prepare-media: $(find "$out" -type f | wc -l | tr -d ' ') files in $out"
