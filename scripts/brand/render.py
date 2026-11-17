#!/usr/bin/env python3
# Adapted from jplayer-mobile/scripts/brand/render.py at 49de3c2.
"""Render an SVG or HTML file of docs/brand to a PNG with Playwright's Chromium.

Usage (from the repository root, with the venv of docs/brand/BRAND.md 1):

  local/.venv/bin/python -I scripts/brand/render.py SRC W H OUT.png [--opaque|--alpha]

SRC is an .svg file (scaled to W x H pixels through its viewBox) or an .html
file (optionally `file.html?query`, drawn in a W x H window). The ground is
transparent where the file paints nothing. The picture is then re-encoded with
Pillow with fixed settings, so the same sources give the same bytes:

  --opaque  RGB, no alpha channel (the iOS icon, the feature graphic, the tvOS
            back layers and Top Shelf images)
  --alpha   RGBA, alpha kept (default); a fully opaque RGBA stays RGBA
"""

import io
import pathlib
import sys

from PIL import Image
from playwright.sync_api import sync_playwright


def render(src: str, width: int, height: int) -> bytes:
    path, _, query = src.partition("?")
    uri = pathlib.Path(path).resolve().as_uri() + ("?" + query if query else "")
    with sync_playwright() as p:
        browser = p.chromium.launch(args=["--force-color-profile=srgb", "--font-render-hinting=none"])
        page = browser.new_page(viewport={"width": width, "height": height}, device_scale_factor=1)
        page.goto(uri)
        if path.endswith(".svg"):
            page.evaluate(
                "([w, h]) => { const e = document.documentElement;"
                " e.setAttribute('width', w); e.setAttribute('height', h);"
                " document.documentElement.style.background = 'transparent'; }",
                [width, height],
            )
        page.evaluate("document.fonts.ready")
        page.wait_for_load_state("load")
        png = page.screenshot(omit_background=True, clip={"x": 0, "y": 0, "width": width, "height": height})
        browser.close()
    return png


def main(argv: list[str]) -> int:
    args = [a for a in argv if not a.startswith("--")]
    flags = {a for a in argv if a.startswith("--")}
    if len(args) != 4 or not flags <= {"--opaque", "--alpha"}:
        print(__doc__, file=sys.stderr)
        return 2
    src, w, h, out = args[0], int(args[1]), int(args[2]), args[3]
    img = Image.open(io.BytesIO(render(src, w, h)))
    img = img.convert("RGB" if "--opaque" in flags else "RGBA")
    pathlib.Path(out).parent.mkdir(parents=True, exist_ok=True)
    img.save(out, format="PNG", optimize=True, compress_level=9)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
