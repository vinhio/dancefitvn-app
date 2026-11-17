#!/bin/sh
# Downloads the font sources into local/fonts/source/ and checks their
# sha256 against the ones below, which assets/fonts/README.md lists too
# (Q-P0-8: the sources stay out of the repository). Run from anywhere.
#
# The sources are google/fonts, branch main, folder ofl/: the static
# Barlow Condensed Bold and ExtraBold, and the variable Inter and
# JetBrains Mono that scripts/fonts/make-static-fonts.sh cuts.
#
# Usage: scripts/fonts/fetch.sh
set -eu

root=$(cd "$(dirname "$0")/../.." && pwd)
dst="$root/local/fonts/source"
base=https://raw.githubusercontent.com/google/fonts/main/ofl
mkdir -p "$dst"

# fetch <url> <file> <sha256>: downloads url to file unless it is there,
# then checks its sha256.
fetch() {
	if [ ! -f "$dst/$2" ]; then
		curl -fsSL -o "$dst/$2" "$1"
	fi
	echo "$3  $dst/$2" | shasum -a 256 -c -
}

fetch "$base/barlowcondensed/BarlowCondensed-Bold.ttf" BarlowCondensed-Bold.ttf \
	e476562ec9c1e16cf16475895b511f08c804f438cc9a9f80a44ea50a0eeb5b65
fetch "$base/barlowcondensed/BarlowCondensed-ExtraBold.ttf" BarlowCondensed-ExtraBold.ttf \
	724c9c25952d5f4a2d87185d9767aa006144c5f0d944dc05bf7d5d603551c260
fetch "$base/barlowcondensed/OFL.txt" OFL-BarlowCondensed.txt \
	186d750eb496a4c17a76385f82be6aea2ac1cf2de074a811d63786cf374ea73f
fetch "$base/inter/Inter%5Bopsz,wght%5D.ttf" 'Inter[opsz,wght].ttf' \
	29160a80ff49ddcab2c97711247e08b1fab27a484a329ce8b813d820dc559031
fetch "$base/inter/OFL.txt" OFL-Inter.txt \
	5b9321a4298cfeb6b34354164a1c3afc3db114569984c502b9b35d988fd58c57
fetch "$base/jetbrainsmono/JetBrainsMono%5Bwght%5D.ttf" 'JetBrainsMono[wght].ttf' \
	48715a42ec242c21e9f02692891e147d022299a52e48d5e413e1a942193ffeda
fetch "$base/jetbrainsmono/OFL.txt" OFL-JetBrainsMono.txt \
	b2fe5e8987594e9ffd1d2ca52a2f5d73eb8335243893c5d6254b5ad69269591d
