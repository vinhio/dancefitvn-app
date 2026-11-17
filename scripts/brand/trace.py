#!/usr/bin/env python3
"""Trace the DanceFIT wordmark of the site logo into an SVG path.

Usage (from the repository root, with the venv of docs/brand/BRAND.md 1):

  local/.venv/bin/python -I scripts/brand/trace.py SRC.png OUT.svg [--no-reg]

SRC is docs/brand/source/dancefitvn.png, the site's logo: the pill mark on
the left, then the black wordmark "DanceFIT" and the registered sign, with a
soft drop shadow down and to the right. The tracer keeps the black glyphs
right of the mark, removes the shadow (its alpha never exceeds SHADOW), scales
the coverage up SCALE times, follows the 50 % contour with marching squares
and simplifies it (Douglas-Peucker, TOLERANCE px of the source). The output is
an even-odd path in white (the site's own white variant of the wordmark) in
the source's pixel units, its viewBox the glyphs' box; --no-reg leaves out the
registered sign. The same source gives the same bytes.
"""

import math
import pathlib
import sys

from PIL import Image

# MARK_RIGHT is the first column right of the pill mark in the source.
MARK_RIGHT = 262
# SHADOW is the drop shadow's highest alpha (measured: 77 of 255).
SHADOW = 77
# SCALE is how many times the coverage is enlarged before tracing.
SCALE = 4
# TOLERANCE is the simplification's error bound, in source pixels.
TOLERANCE = 0.15


def coverage(path: str) -> tuple[Image.Image, tuple[int, int]]:
    """Return the glyphs' coverage (L mode, shadow removed) and its offset."""
    im = Image.open(path).convert("RGBA")
    w, h = im.size
    cov = Image.new("L", (w - MARK_RIGHT, h), 0)
    px, out = im.load(), cov.load()
    for y in range(h):
        for x in range(MARK_RIGHT, w):
            r, g, b, a = px[x, y]
            if max(r, g, b) < 96 and a > SHADOW:
                out[x - MARK_RIGHT, y] = min(255, round((a - SHADOW) * 255 / (255 - SHADOW)))
    box = cov.getbbox()
    pad = 2
    cov = cov.crop((box[0] - pad, box[1] - pad, box[2] + pad, box[3] + pad))
    return cov, (MARK_RIGHT + box[0] - pad, box[1] - pad)


def contours(img: Image.Image) -> list[list[tuple[float, float]]]:
    """Follow the 50 % contour of img with marching squares; closed loops."""
    w, h = img.size
    v = [[img.getpixel((x, y)) / 255.0 for x in range(w)] for y in range(h)]
    iso = 0.5

    def point(edge):
        kind, x, y = edge
        if kind == "h":  # between (x, y) and (x + 1, y)
            a, b = v[y][x], v[y][x + 1]
            return x + (iso - a) / (b - a), y
        a, b = v[y][x], v[y + 1][x]
        return x, y + (iso - a) / (b - a)

    links: dict = {}

    def link(e1, e2):
        links.setdefault(e1, []).append(e2)
        links.setdefault(e2, []).append(e1)

    for y in range(h - 1):
        for x in range(w - 1):
            tl, tr, br, bl = v[y][x] >= iso, v[y][x + 1] >= iso, v[y + 1][x + 1] >= iso, v[y + 1][x] >= iso
            top, right, bottom, left = ("h", x, y), ("v", x + 1, y), ("h", x, y + 1), ("v", x, y)
            case = tl * 8 + tr * 4 + br * 2 + bl
            if case in (0, 15):
                continue
            if case in (5, 10):
                centre = (v[y][x] + v[y][x + 1] + v[y + 1][x + 1] + v[y + 1][x]) / 4 >= iso
                if (case == 5) == centre:
                    link(top, right)
                    link(bottom, left)
                else:
                    link(top, left)
                    link(bottom, right)
                continue
            crossed = [e for e, s in ((top, tl != tr), (right, tr != br), (bottom, br != bl), (left, bl != tl)) if s]
            link(crossed[0], crossed[1])

    loops, seen = [], set()
    for start in sorted(links):
        if start in seen:
            continue
        loop, prev, cur = [], None, start
        while cur not in seen:
            seen.add(cur)
            loop.append(point(cur))
            nxt = [e for e in links[cur] if e != prev and e not in seen]
            if not nxt:
                break
            prev, cur = cur, nxt[0]
        if len(loop) > 2:
            loops.append(loop)
    return loops


def simplify(pts: list[tuple[float, float]], tol: float) -> list[tuple[float, float]]:
    """Douglas-Peucker on a closed loop, split at its two farthest points."""

    def dp(seg):
        if len(seg) < 3:
            return seg
        (x0, y0), (x1, y1) = seg[0], seg[-1]
        dx, dy = x1 - x0, y1 - y0
        n = math.hypot(dx, dy) or 1e-9
        worst, at = -1.0, 0
        for i in range(1, len(seg) - 1):
            d = abs((seg[i][0] - x0) * dy - (seg[i][1] - y0) * dx) / n
            if d > worst:
                worst, at = d, i
        if worst <= tol:
            return [seg[0], seg[-1]]
        return dp(seg[: at + 1])[:-1] + dp(seg[at:])

    far = max(range(len(pts)), key=lambda i: (pts[i][0] - pts[0][0]) ** 2 + (pts[i][1] - pts[0][1]) ** 2)
    a = dp(pts[: far + 1])
    b = dp(pts[far:] + [pts[0]])
    return a[:-1] + b[:-1]


def is_reg(loop: list[tuple[float, float]], right: float, height: float) -> bool:
    """Report whether a loop belongs to the registered sign: high, past the T."""
    cx = sum(p[0] for p in loop) / len(loop)
    cy = sum(p[1] for p in loop) / len(loop)
    return cx > right - 0.4 * height and cy < 0.45 * height


def main(argv: list[str]) -> int:
    args = [a for a in argv if not a.startswith("--")]
    flags = {a for a in argv if a.startswith("--")}
    if len(args) != 2 or not flags <= {"--no-reg"}:
        print(__doc__, file=sys.stderr)
        return 2
    cov, (ox, oy) = coverage(args[0])
    big = cov.resize((cov.width * SCALE, cov.height * SCALE), Image.LANCZOS)
    loops = [simplify(lp, TOLERANCE * SCALE) for lp in contours(big)]
    if "--no-reg" in flags:
        loops = [lp for lp in loops if not is_reg(lp, big.width, big.height)]
    xs = [p[0] / SCALE for lp in loops for p in lp]
    ys = [p[1] / SCALE for lp in loops for p in lp]
    x0, y0 = math.floor(min(xs) * 100) / 100, math.floor(min(ys) * 100) / 100
    w, h = math.ceil((max(xs) - x0) * 100) / 100, math.ceil((max(ys) - y0) * 100) / 100

    def fmt(n: float) -> str:
        s = f"{n:.2f}".rstrip("0").rstrip(".")
        return "0" if s == "-0" else s

    d = []
    for lp in loops:
        d.append("M" + "L".join(f"{fmt(p[0] / SCALE - x0)} {fmt(p[1] / SCALE - y0)}" for p in lp) + "Z")
    what = "without the registered sign" if "--no-reg" in flags else "with the registered sign"
    svg = (
        f'<svg width="{fmt(w)}" height="{fmt(h)}" viewBox="0 0 {fmt(w)} {fmt(h)}" xmlns="http://www.w3.org/2000/svg" '
        'role="img" aria-label="DanceFIT">\n'
        f"  <!-- The DanceFIT wordmark, {what}, traced from docs/brand/source/dancefitvn.png"
        f" by scripts/brand/trace.py: source pixels, origin at ({fmt(ox + x0)}, {fmt(oy + y0)}) of the source."
        " Do not edit: rerun the tracer. -->\n"
        f'  <path fill="#FFFFFF" fill-rule="evenodd" d="{"".join(d)}"/>\n'
        "</svg>\n"
    )
    pathlib.Path(args[1]).parent.mkdir(parents=True, exist_ok=True)
    pathlib.Path(args[1]).write_text(svg, encoding="utf-8")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
