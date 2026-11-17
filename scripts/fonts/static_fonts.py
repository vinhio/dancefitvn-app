#!/usr/bin/env python3
# Adapted from jplayer-mobile/scripts/fonts/static_fonts.py at 49de3c2.
"""Name a static instance cut from a variable font, and list the faces.

Used by scripts/fonts/make-static-fonts.sh (P0.5, PLAN D5):

  static_fonts.py name  IN OUT FAMILY STYLE WEIGHT PSNAME
  static_fonts.py sums  DIR

`name` rewrites the name table of IN so that Core Text finds the face by its
PostScript name (ID 6) and gotext by its typographic family (ID 16) and style
(ID 17), sets OS/2 usWeightClass, drops the variation tables, and writes OUT.
`sums` prints the sha256 of every .ttf in DIR, one per line, as shasum does.
"""
import hashlib
import sys
from pathlib import Path

from fontTools.ttLib import TTFont


def set_name(font, name_id, value):
    """Sets name_id on the Windows and Mac platforms."""
    table = font["name"]
    table.removeNames(nameID=name_id)
    table.setName(value, name_id, 3, 1, 0x409)
    table.setName(value, name_id, 1, 0, 0)


def name_instance(src, dst, family, style, weight, ps):
    font = TTFont(src, recalcTimestamp=False)  # keep the source's dates: the output is reproducible
    full = f"{family} {style}"
    for name_id in (21, 22, 25):  # legacy WWS and variations PostScript prefix
        font["name"].removeNames(nameID=name_id)
    set_name(font, 1, full)
    set_name(font, 2, "Regular")
    set_name(font, 3, f"{ps};static")
    set_name(font, 4, full)
    set_name(font, 6, ps)
    set_name(font, 16, family)
    set_name(font, 17, style)
    os2 = font["OS/2"]
    os2.usWeightClass = int(weight)
    # The legacy family holds one face, "Regular": mark it so.
    os2.fsSelection = (os2.fsSelection & ~0b100001) | 0b1000000
    font["head"].macStyle = 0
    font["head"].modified = font["head"].created  # reproducible output
    for tag in ("STAT", "fvar", "avar", "gvar", "HVAR", "MVAR", "VVAR"):
        if tag in font:
            del font[tag]
    font.save(dst)


def sums(directory):
    for path in sorted(Path(directory).glob("*.ttf")):
        print(f"{hashlib.sha256(path.read_bytes()).hexdigest()}  {path.name}")


def main(argv):
    if len(argv) == 8 and argv[1] == "name":
        name_instance(*argv[2:])
    elif len(argv) == 3 and argv[1] == "sums":
        sums(argv[2])
    else:
        sys.exit(__doc__)


if __name__ == "__main__":
    main(sys.argv)
