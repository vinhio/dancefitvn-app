#!/usr/bin/env python3
"""Print every CSS rule of the prototype as Markdown tables.

docs/plan/02-ui-spec.md §4 is this script's output, pasted as is; run it
again whenever reference/prototype/index.html changes and diff. Rules are
grouped by the CSS section comments (shared atoms, mobile, video stage,
TV). The simulator's own chrome (.bar, .stage, .phone, .remote …) is
listed apart, as it is not part of the app (K3).

Usage: python3 reference/tools/css_spec.py > /tmp/css.md
"""
import os
import re

CHROME = {".app", ".bar", ".brand", ".brand small", ".seg", ".seg button", ".seg button.on", ".tog", ".tog i",
          ".tog i:after", ".tog.on i", ".tog.on i:after", ".reset", ".note", ".stage", ".col", ".cap", ".holder",
          ".phone", ".phone .island", ".tvbox", ".tvframe", ".tvstand", ".remote", ".remote button",
          ".remote button:hover", ".remote span", "html,body", "*", "button", "input,textarea"}


def main():
    p = os.path.join(os.path.dirname(__file__), "..", "prototype", "index.html")
    s = open(p, encoding="utf-8").read()
    css = s[s.find("<style>") + 7:s.find("</style>")]
    section = "Tokens and base"
    groups = {section: []}
    chrome = []
    tok = re.compile(r"@keyframes\s+([\w-]+)\s*\{((?:[^{}]|\{[^{}]*\})*)\}|([^{}]+)\{([^{}]*)\}", re.S)
    parts = re.split(r"/\*\s*-*\s*(.*?)\s*-*\s*\*/", css, flags=re.S)
    for i, chunk in enumerate(parts):
        if i % 2 == 1:
            section = chunk.strip().capitalize()
            groups.setdefault(section, [])
            continue
        for m in tok.finditer(chunk):
            if m.group(1):
                groups[section].append(("@keyframes " + m.group(1), re.sub(r"\s+", " ", m.group(2)).strip()))
                continue
            sel = re.sub(r"\s+", " ", m.group(3)).strip()
            body = re.sub(r"\s+", " ", m.group(4)).strip().rstrip(";")
            if sel in CHROME:
                chrome.append(sel)
                continue
            groups[section].append((sel, body))
    for g, rules in groups.items():
        if not rules:
            continue
        print(f"\n### {g}\n")
        print("| Selector | Declarations |\n|---|---|")
        for sel, body in rules:
            body = body.replace("|", "\\|")
            print(f"| `{sel}` | {body.replace(';', '; ')} |")
    print("\nSimulator chrome, not built (K3): " + ", ".join(f"`{c}`" for c in chrome))


if __name__ == "__main__":
    main()
