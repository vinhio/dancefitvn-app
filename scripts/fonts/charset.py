#!/usr/bin/env python3
# Adapted from jplayer-mobile/scripts/fonts/charset.py at 49de3c2.
"""Write the characters the embedded fonts are subset to, and audit them (P0.5).

Usage: charset.py OUT

OUT gets one "U+XXXX" per line (pyftsubset --unicodes-file): the union of

  - every value of locales/vi.json, and every string of local/strings.json
    (the prototype's strings, reference/tools extract_strings) when present;
  - U+0020-007E, U+00A0-00FF (Latin-1), the letters of Latin Extended-A and
    -B Vietnamese writes (Ă ă Đ đ Ĩ ĩ Ũ ũ Ơ ơ Ư ư), every combining mark
    of decomposed (NFD) Vietnamese: the tone marks U+0300, U+0301, U+0303,
    U+0309, U+0323, the circumflex U+0302, the breve U+0306 and the horn
    U+031B (a mark the app's NFC composition, format.NFC, leaves standing
    then still has a glyph on Android), U+1EA0-1EF9 (the Vietnamese letters of
    Latin Extended Additional), U+2000-206F (general punctuation), U+20AB
    (dong), U+20AC (euro), U+2122, U+2190-2193 (arrows), U+2212 (minus),
    U+00D7, U+2026, U+00B7, U+25CC (the dotted circle shapers put under a
    lone mark).

It then prints, for review, the characters the prototype draws that are
outside the set (those of reference/prototype/index.html, markup and
scripts). It also prints the characters of the set's strings no source font
has (they fall back on the system's fonts whatever the subset). It exits
non-zero when a value of vi.json has a character outside the set, which
cannot happen unless this script is broken.
"""
import json
import re
import sys
import unicodedata
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]

RANGES = [
    (0x0020, 0x007E), (0x00A0, 0x00FF),
    (0x0102, 0x0103), (0x0110, 0x0111), (0x0128, 0x0129), (0x0168, 0x0169),
    (0x01A0, 0x01A1), (0x01AF, 0x01B0),
    (0x0300, 0x0303), (0x0306, 0x0306), (0x0309, 0x0309), (0x031B, 0x031B),
    (0x0323, 0x0323),
    (0x1EA0, 0x1EF9),
    (0x2000, 0x206F), (0x20AB, 0x20AB), (0x20AC, 0x20AC), (0x2122, 0x2122),
    (0x2190, 0x2193), (0x2212, 0x2212), (0x00D7, 0x00D7), (0x2026, 0x2026),
    (0x00B7, 0x00B7), (0x25CC, 0x25CC),
]

# Control characters are never drawn.
SKIP = {"\n", "\r", "\t"}


def strings(node):
    """Yields every string (keys and values) of a JSON tree."""
    if isinstance(node, str):
        yield node
    elif isinstance(node, dict):
        for k, v in node.items():
            yield k
            yield from strings(v)
    elif isinstance(node, list):
        for v in node:
            yield from strings(v)


def texts(vi):
    """Returns the app's strings: vi.json's values and the prototype's."""
    out = list(vi.values())
    extracted = ROOT / "local/strings.json"
    if extracted.exists():
        out += strings(json.loads(extracted.read_text()))
    return out


def charset(vi):
    """Returns the set of code points to keep."""
    keep = set()
    for lo, hi in RANGES:
        keep.update(range(lo, hi + 1))
    for t in texts(vi):
        for c in unicodedata.normalize("NFC", t):
            if c not in SKIP:
                keep.add(ord(c))
    return keep


def drawn():
    """Returns {character: [where]} for what the prototype may draw."""
    found = {}
    path = ROOT / "reference/prototype/index.html"
    src = path.read_text()
    # \uXXXX escapes in the JavaScript are characters too.
    src = re.sub(r"\\u([0-9a-fA-F]{4})", lambda m: chr(int(m.group(1), 16)), src)
    for c in unicodedata.normalize("NFC", src):
        if c not in SKIP:
            found.setdefault(c, set()).add(path.relative_to(ROOT).as_posix())
    return found


def font_gaps(chars):
    """Returns {font: characters of chars it lacks} for the full faces."""
    try:
        from fontTools.ttLib import TTFont
    except ImportError:
        return None
    out = {}
    for p in sorted((ROOT / "local/fonts/static").glob("*.ttf")):
        cmap = TTFont(p, lazy=True).getBestCmap()
        out[p.name] = sorted(c for c in chars if ord(c) not in cmap)
    return out


def name(c):
    return f"U+{ord(c):04X} {c!r} {unicodedata.name(c, '?')}"


def main(argv):
    if len(argv) != 2:
        sys.exit(__doc__)
    vi = json.loads((ROOT / "locales/vi.json").read_text())
    keep = charset(vi)
    Path(argv[1]).write_text("".join(f"U+{cp:04X}\n" for cp in sorted(keep)))
    print(f"charset: {len(keep)} code points -> {argv[1]}")

    outside = {c: w for c, w in drawn().items() if ord(c) not in keep}
    print(f"in reference/prototype/index.html, outside the set: {len(outside)}")
    for c in sorted(outside):
        print(f"  {name(c)}")

    text_chars = {c for t in texts(vi) for c in unicodedata.normalize("NFC", t)} - SKIP
    gaps = font_gaps(text_chars)
    if gaps is not None:
        for font, missing in gaps.items():
            if missing:
                print(f"app strings {font} lacks: " + " ".join(name(c) for c in missing))

    vi_chars = {c for t in vi.values() for c in unicodedata.normalize("NFC", t)} - SKIP
    bad = sorted(c for c in vi_chars if ord(c) not in keep)
    if bad:
        print("vi.json characters outside the set: " + " ".join(name(c) for c in bad))
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
