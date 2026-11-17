#!/bin/sh
# Regenerates <name>.<L|M|Q|H>.txt (libqrencode matrices, '#' dark, '.' light)
# from <name>.in. Needs qrencode (brew install qrencode). Run by hand.
set -e
cd "$(dirname "$0")"
for in in *.in; do
	name=${in%.in}
	for l in L M Q H; do
		qrencode -t ASCII -l "$l" -8 -m 0 -r "$in" -o - |
			sed -e 's/##/#/g' -e 's/  /./g' >"$name.$l.txt"
	done
done
