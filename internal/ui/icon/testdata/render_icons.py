#!/usr/bin/env python3
"""Render every icon of the prototype to testdata/proto/<name>.png.

Adapted from jplayer-tv/internal/ui/icon/testdata/render_icons.py at 781bee5.

Each icon is drawn the way the prototype draws it: ic(n,24) of
reference/prototype/index.html (index.html:239), an <svg class="i"> with the
rule `svg.i` (index.html:64: stroke currentColor, fill none, stroke-width 1.9,
round caps and joins), white on black, 24x24 CSS px at device scale 3 (72x72
files). PLAY and PAUSE are the filled glyphs of index.html:240-241 and the
triangle is the lesson row's (index.html:392). A few icons are drawn again at
the stroke widths the screens ask for: home 1.8 and 2.2 (the tab bar,
index.html:324), dl 2 and 2.2, cr 2.

    local/.venv/bin/python internal/ui/icon/testdata/render_icons.py
"""
import pathlib
import re

from playwright.sync_api import sync_playwright

HERE = pathlib.Path(__file__).resolve().parent
ROOT = HERE.parents[3]
INDEX = ROOT / "reference" / "prototype" / "index.html"
OUT = HERE / "proto"
SIZE = 24
SCALE = 3

html = INDEX.read_text(encoding="utf-8")
table = re.search(r"const IC=\{(.*?)\};", html, re.S).group(1)
icons = re.findall(r"([a-z0-9]+):'([^']+)'", table)
assert len(icons) == 42, len(icons)
play = re.search(r"const PLAY='<svg[^>]*><path d=\"([^\"]+)\"", html).group(1)
pause = re.search(r"const PAUSE='<svg[^>]*><path d=\"([^\"]+)\"", html).group(1)
tri = re.search(r'<path d="(M6 4l14 8-14 8z)" fill="\$\{cur', html).group(1)

# svg.i (index.html:64), at the size ic(n,24) gives it.
css = f"""
html, body {{ margin: 0; background: #000; }}
.box {{ width: {SIZE}px; height: {SIZE}px; background: #000; color: #fff; }}
svg.i {{ width: {SIZE}px; height: {SIZE}px; display: block; stroke: currentColor; fill: none;
  stroke-width: 1.9; stroke-linecap: round; stroke-linejoin: round; flex: none; }}
svg.f {{ width: {SIZE}px; height: {SIZE}px; display: block; }}
"""
cells = []
names = []


def add(name, svg):
    names.append(name)
    cells.append(f'<div class="box" id="b{len(names) - 1}">{svg}</div>')


for n, d in icons:
    add(n, f'<svg class="i" viewBox="0 0 24 24"><path d="{d}"/></svg>')
for n, w in [("home", "1.8"), ("home", "2.2"), ("dl", "2"), ("dl", "2.2"), ("cr", "2")]:
    d = dict(icons)[n]
    add(f"{n}@{w}", f'<svg class="i" viewBox="0 0 24 24" style="stroke-width:{w}"><path d="{d}"/></svg>')
add("play", f'<svg class="f" viewBox="0 0 24 24"><path d="{play}" fill="currentColor"/></svg>')
add("pause", f'<svg class="f" viewBox="0 0 24 24"><path d="{pause}" fill="currentColor"/></svg>')
add("tri", f'<svg class="f" viewBox="0 0 24 24"><path d="{tri}" fill="none" stroke="currentColor" stroke-width="2"/></svg>')
add("trifill", f'<svg class="f" viewBox="0 0 24 24"><path d="{tri}" fill="currentColor" stroke="currentColor" stroke-width="2"/></svg>')

page_html = f"<!doctype html><style>{css}</style><body>{''.join(cells)}</body>"
OUT.mkdir(parents=True, exist_ok=True)
with sync_playwright() as p:
    browser = p.chromium.launch()
    page = browser.new_page(viewport={"width": 200, "height": 200}, device_scale_factor=SCALE)
    page.set_content(page_html)
    for i, n in enumerate(names):
        page.locator(f"#b{i}").screenshot(path=str(OUT / f"{n}.png"))
    browser.close()
print(len(names), "icons")
