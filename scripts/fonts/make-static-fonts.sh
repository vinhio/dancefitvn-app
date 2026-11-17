#!/usr/bin/env bash
# Adapted from jplayer-mobile/scripts/fonts/make-static-fonts.sh at 49de3c2.
#
# Cuts the static faces DanceFITVN draws in (PLAN D5) from the sources that
# scripts/fonts/fetch.sh downloads into local/fonts/source, into
# local/fonts/static (the full faces; scripts/fonts/subset.sh then subsets
# them into assets/fonts):
#   Inter           wght 400, 500, 600, 700, 800 at opsz 14, the variable
#                   font's default and the optical size of the woff2 the
#                   prototype's Google Fonts CSS serves (a wght axis alone);
#   JetBrains Mono  wght 500, 700;
#   Barlow Condensed Bold and ExtraBold, static upstream: copied unchanged.
# Each cut is named so that its PostScript name is its file's stem
# (static_fonts.py name).
#
# Changes from jPlayer Mobile: the families, no optical size groups, the
# sources in local/ (Q-P0-8), the venv local/.venv.
#
# fonttools comes from the venv FONT_VENV (default local/.venv), and is
# installed into it when missing. The subset files are committed, so a
# normal build never needs it.
#
# Usage: scripts/fonts/make-static-fonts.sh
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
src="$root/local/fonts/source"
dst="$root/local/fonts/static"
venv="${FONT_VENV:-$root/local/.venv}"

if [[ ! -x "$venv/bin/python" ]]; then
  python3 -m venv "$venv"
fi
if ! "$venv/bin/python" -I -c 'import fontTools' 2>/dev/null; then
  "$venv/bin/pip" install 'fonttools>=4.50' brotli
fi
sh "$root/scripts/fonts/fetch.sh" >/dev/null

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$dst"
rm -f "$dst"/*.ttf

# cut_face <variable file> <family> <wght> <style> <extra axes...>
cut_face() {
  local file="$1" family="$2" wght="$3" style="$4"
  shift 4
  local ps="${family// /}-${style}"
  "$venv/bin/fonttools" varLib.instancer -q -o "$tmp/$ps.ttf" "$src/$file" "wght=$wght" "$@"
  "$venv/bin/python" -I "$root/scripts/fonts/static_fonts.py" name \
    "$tmp/$ps.ttf" "$dst/$ps.ttf" "$family" "$style" "$wght" "$ps"
  echo "cut   $ps.ttf"
}

inter='Inter[opsz,wght].ttf'
mono='JetBrainsMono[wght].ttf'

cut_face "$inter" 'Inter' 400 Regular opsz=14
cut_face "$inter" 'Inter' 500 Medium opsz=14
cut_face "$inter" 'Inter' 600 SemiBold opsz=14
cut_face "$inter" 'Inter' 700 Bold opsz=14
cut_face "$inter" 'Inter' 800 ExtraBold opsz=14
cut_face "$mono" 'JetBrains Mono' 500 Medium
cut_face "$mono" 'JetBrains Mono' 700 Bold
for f in BarlowCondensed-Bold.ttf BarlowCondensed-ExtraBold.ttf; do
  cp "$src/$f" "$dst/$f"
  echo "copy  $f"
done

"$venv/bin/python" -I "$root/scripts/fonts/static_fonts.py" sums "$dst"
