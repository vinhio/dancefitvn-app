#!/usr/bin/env python3
"""List the user-facing strings of the prototype.

Reads reference/prototype/index.html, drops CSS, JS comments and SVG, and
collects:
  * text nodes of the HTML and of the JS template literals (between > and <),
  * quoted JS strings that hold a letter and a space or a Vietnamese letter,
  * placeholder="…" and aria-label="…" values.
Template holes (${…}) are cut out, so "Bài ${n}" yields "Bài" pieces only;
those strings are reviewed by hand (docs/plan/03-behavior-spec.md §8 lists
the formatted ones).

Output: JSON list, sorted, unique -> stdout or --out FILE.
The app's locale test checks every value of locales/vi.json against this
list (or against the formatted list of 03 §8).
"""
import argparse
import html as H
import json
import os
import re

VI = re.compile(r"[ăâđêôơưàáạảãằắặẳẵầấậẩẫèéẹẻẽềếệểễìíịỉĩòóọỏõồốộổỗờớợởỡùúụủũừứựửữỳýỵỷỹ]", re.I)
CODEISH = re.compile(r"(^[#.][\w-]|\$\{|=>|\bfunction\b|\breturn\b|[{};]|\.\w+\(|^\s*[a-z]+[A-Z]\w*\s*$|^#[0-9a-f]{3,8}$|px\b|rgba?\(|var\(--)")


def keep(s):
    s = H.unescape(re.sub(r"\s+", " ", s)).strip(" ·›‹")
    if len(s) < 2 or not re.search(r"[A-Za-zÀ-ỹ]", s):
        return None
    if CODEISH.search(s):
        return None
    if not (VI.search(s) or " " in s or s[0].isupper()):
        return None
    return s


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--proto", default=os.path.join(os.path.dirname(__file__), "..", "prototype", "index.html"))
    ap.add_argument("--out")
    a = ap.parse_args()
    src = open(a.proto, encoding="utf-8").read()
    src = re.sub(r"<style>.*?</style>", "", src, flags=re.S)
    # The static body is the simulator's chrome (view switch, remote,
    # captions); every app string is built by the script.
    src = re.sub(r"<body>.*?<script>", "<script>", src, flags=re.S)
    src = re.sub(r"<svg.*?</svg>", "", src, flags=re.S)
    src = re.sub(r"/\*.*?\*/", "", src, flags=re.S)
    src = re.sub(r"(?m)^\s*//.*$", "", src)
    out = set()
    holes = re.sub(r"\$\{[^{}]*(\{[^{}]*\}[^{}]*)*\}", "\x00", src)
    for m in re.finditer(r">([^<>]+)<", holes):
        for part in m.group(1).split("\x00"):
            k = keep(part)
            if k:
                out.add(k)
    for m in re.finditer(r"(?:placeholder|aria-label|title)=\"([^\"]+)\"", src):
        k = keep(m.group(1))
        if k:
            out.add(k)
    for m in re.finditer(r"'([^'\n]{2,})'", holes):
        s = m.group(1)
        if "<" in s or ">" in s:
            continue
        for part in s.split("\x00"):
            k = keep(part)
            if k and (VI.search(k) or " " in k):
                out.add(k)
    res = sorted(out)
    txt = json.dumps(res, ensure_ascii=False, indent=1)
    if a.out:
        open(a.out, "w", encoding="utf-8").write(txt)
    else:
        print(txt)
    print(len(res), "strings", file=__import__("sys").stderr)


if __name__ == "__main__":
    main()
