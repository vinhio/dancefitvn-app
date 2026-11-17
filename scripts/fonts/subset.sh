#!/usr/bin/env bash
# Adapted from jplayer-mobile/scripts/fonts/subset.sh at 49de3c2.
#
# Subsets the full faces of local/fonts/static (scripts/fonts/
# make-static-fonts.sh) to the characters of scripts/fonts/unicodes.txt
# (Latin and Vietnamese, the app's strings: see charset.py) into
# assets/fonts, which the app embeds (assets/assets.go); copies the
# licences there; and writes assets/fonts/SHA256SUMS.txt and
# assets/fonts/README.md: the source, the licence, and the size and sha256
# of every file before and after the subset (P0.5, PLAN D5, Q-P0-8).
#
# The subset keeps every layout feature (kerning, marks, ligatures, tabular
# figures), every name record (the names Core Text and gotext find the
# faces by are those of the full font), the hinting and the outline of
# .notdef, and keeps the timestamps, so the output is reproducible.
#
# Changes from jPlayer Mobile: the paths (sources in local/, Q-P0-8), the
# venv local/.venv, the README's text and its table of sources.
#
# Usage: scripts/fonts/subset.sh [--charset]
#   --charset  runs charset.py first, rewriting unicodes.txt from the current
#              locales/vi.json and local/strings.json and printing what falls
#              outside it.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
srcdir="$root/local/fonts/source"
src="$root/local/fonts/static"
dst="$root/assets/fonts"
unicodes="$root/scripts/fonts/unicodes.txt"
venv="${FONT_VENV:-$root/local/.venv}"
base=https://raw.githubusercontent.com/google/fonts/main/ofl

if ! "$venv/bin/python" -I -c 'import fontTools' 2>/dev/null; then
  echo "fonttools missing in $venv: run scripts/fonts/make-static-fonts.sh first" >&2
  exit 1
fi
if [[ "${1:-}" == "--charset" ]]; then
  "$venv/bin/python" -I "$root/scripts/fonts/charset.py" "$unicodes"
fi

size() { stat -f %z "$1" 2>/dev/null || stat -c %s "$1"; }
sum() { shasum -a 256 "$1" | cut -d' ' -f1; }

rm -f "$dst"/*.ttf "$dst"/OFL-*.txt
for f in OFL-BarlowCondensed.txt OFL-Inter.txt OFL-JetBrainsMono.txt; do
  cp "$srcdir/$f" "$dst/$f"
done
before=0
after=0
rows=""
for f in "$src"/*.ttf; do
  name="$(basename "$f")"
  "$venv/bin/pyftsubset" "$f" \
    --output-file="$dst/$name" \
    --unicodes-file="$unicodes" \
    --layout-features='*' \
    --name-IDs='*' --name-languages='*' --name-legacy \
    --notdef-outline \
    --no-recalc-timestamp
  b="$(size "$f")"
  a="$(size "$dst/$name")"
  before=$((before + b))
  after=$((after + a))
  printf '%-32s %8d -> %8d  (%3d%%)\n' "$name" "$b" "$a" $((a * 100 / b))
  rows+="| \`$name\` | $b | \`$(sum "$f")\` | $a | \`$(sum "$dst/$name")\` |"$'\n'
done
printf '%-32s %8d -> %8d  (%3d%%)\n' "total" "$before" "$after" $((after * 100 / before))

# The sums the app names its cached copies by, so that it need not hash the
# fonts as it starts (internal/app/fonts.go).
(cd "$dst" && shasum -a 256 *.ttf) > "$dst/SHA256SUMS.txt"

{
  cat <<'EOF'
# Fonts

The faces DanceFITVN draws in (PLAN D5), embedded by `assets.Fonts` (`assets/assets.go`) and
registered at launch by `internal/app/fonts.go`: written once to the user's cache folder, then given to
`auto.RegisterFontFile` (Core Text on iOS and tvOS, gotext on Android), Inter first so that gotext's
default family is the body font. `internal/ui/theme/faces.go` names them: Core Text by PostScript name
at `text.WeightRegular` (its named families collapse every weight from 600 up to the bold trait),
gotext by typographic family and weight.

All are licensed under the SIL Open Font License 1.1; the licence text of each family is the
`OFL-<Family>.txt` file beside it.

- Sources: https://github.com/google/fonts, branch `main`, folder `ofl/`, fetched 2026-11-14 by
  `scripts/fonts/fetch.sh` into `local/fonts/source/` (not committed, Q-P0-8), which checks the sha256
  below.
- `scripts/fonts/make-static-fonts.sh` cuts the static faces into `local/fonts/static/`: Inter at
  wght 400, 500, 600, 700, 800 and opsz 14 (the variable font's default, and what the prototype's
  Google Fonts CSS serves: a woff2 with a `wght` axis alone, whose advances equal the opsz 14 cuts';
  `local/p0/fonts.md`), JetBrains Mono at wght 500 and 700, each renamed so that its PostScript name
  is its file's stem and its typographic family and style are `Inter` / `JetBrains Mono` and the
  style word (`static_fonts.py`); Barlow Condensed Bold and ExtraBold are upstream's static files,
  unchanged (600 is loaded by the prototype but never drawn, Q-P0-7).
- The `*.ttf` files here are those faces subset to Latin and Vietnamese by `scripts/fonts/subset.sh`
  (the characters of `scripts/fonts/unicodes.txt`, written by `scripts/fonts/charset.py` from
  `locales/vi.json`, the prototype's strings and fixed ranges), every layout feature and name kept.
  Rerun `scripts/fonts/subset.sh --charset` when `locales/vi.json` gains a character.
- `SHA256SUMS.txt` lists the sha256 of the embedded fonts: the app names its cached copies by them
  instead of hashing the fonts as it starts.

| Source (`local/fonts/source/`, not embedded) | URL | sha256 | Bytes |
|---|---|---|---|
EOF
  for pair in \
    "BarlowCondensed-Bold.ttf|$base/barlowcondensed/BarlowCondensed-Bold.ttf" \
    "BarlowCondensed-ExtraBold.ttf|$base/barlowcondensed/BarlowCondensed-ExtraBold.ttf" \
    "OFL-BarlowCondensed.txt|$base/barlowcondensed/OFL.txt" \
    "Inter[opsz,wght].ttf|$base/inter/Inter%5Bopsz,wght%5D.ttf" \
    "OFL-Inter.txt|$base/inter/OFL.txt" \
    "JetBrainsMono[wght].ttf|$base/jetbrainsmono/JetBrainsMono%5Bwght%5D.ttf" \
    "OFL-JetBrainsMono.txt|$base/jetbrainsmono/OFL.txt"; do
    file="${pair%%|*}"
    url="${pair#*|}"
    printf '| `%s` | %s | `%s` | %d |\n' "$file" "$url" "$(sum "$srcdir/$file")" "$(size "$srcdir/$file")"
  done
  echo
  echo "| Face | Bytes, full | sha256, full (\`local/fonts/static/\`) | Bytes, subset | sha256, subset (embedded) |"
  echo "|---|---|---|---|---|"
  printf '%s' "$rows"
  printf '| total | %d | | %d | |\n' "$before" "$after"
  echo
  echo "| Licence | sha256 |"
  echo "|---|---|"
  for f in "$dst"/OFL-*.txt; do
    printf '| `%s` | `%s` |\n' "$(basename "$f")" "$(sum "$f")"
  done
} > "$dst/README.md"
