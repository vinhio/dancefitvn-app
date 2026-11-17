#!/usr/bin/env python3
"""Capture every prototype state of DanceFITVN at scale 1.

For each state it writes <out>/<platform>/<ID>.png and <ID>.layout.json
(boxes and key computed styles of every interactive or labelled element).
Mobile states are 393x852, TV states 1920x1080, both at device scale 1
(--dpr 3 gives @3x phone images).

The clock is frozen (page.clock) so timers (toast, splash, playback
ticks) never fire on their own; each state is set through the
prototype's own globals (S, renderM, renderTV, go, tvGo).

Usage:
  cd reference/prototype && python3 -m http.server 5180 &
  TZ=UTC python3 reference/tools/capture_dancefitvn.py \
      --url http://localhost:5180 --out local/shots/prototype [--only M06,TV07]
"""
import argparse
import json
import os
import sys

from playwright.sync_api import sync_playwright

# Each state: (ID, platform, JS run after a fresh initState()).
# The ID matches the screen codes of the design handoff (01..16, TV01..TV12)
# plus suffixes for sheets and variants. Keep in sync with
# docs/plan/07-testing.md section 3.
RESET = "initState();S.auth=true;S.tv.auth=true;"
M = "renderM();"
T = "renderTV();"

STATES = [
    ("M01", "m", "initState();S.m.scr='splash';" + M),
    ("M02", "m", "initState();S.m.scr='login';" + M),
    ("M02B", "m", "initState();S.m.scr='login';S.loginErr=true;" + M),
    ("M03B", "m", RESET + "S.m.scr='loading';" + M),
    ("M03", "m", RESET + "S.m.scr='home';" + M),
    ("M04", "m", RESET + "S.m.scr='courses';" + M),
    ("M04-special", "m", RESET + "S.m.scr='courses';S.m.filter='Đặc biệt';" + M),
    ("M04-empty", "m", RESET + "S.m.scr='courses';S.m.q='zzz';" + M),
    ("M05", "m", RESET + "S.m.scr='detail';S.m.set='set35';" + M),
    ("M05B", "m", RESET + "S.m.scr='locked';" + M),
    ("M06", "m", RESET + "S.m.scr='video';S.pm.kind='video';" + M),
    ("M06-playing", "m", RESET + "S.m.scr='video';S.pm.kind='video';S.pm.playing=true;S.m.ctlT=Date.now();" + M),
    ("M06-hidden", "m", RESET + "S.m.scr='video';S.pm.kind='video';S.pm.playing=true;S.m.ctlHid=true;" + M),
    ("M07", "m", RESET + "S.m.scr='video';S.m.sheet='options';" + M),
    ("M08", "m", RESET + "S.m.scr='audio';S.pm.kind='audio';" + M),
    ("M09", "m", RESET + "S.m.scr='detail';S.m.sheetK='set35-5';S.m.dlChoice='one';S.m.sheet='download';" + M),
    ("M10", "m", RESET + "S.m.scr='done';" + M),
    ("M11", "m", RESET + "S.m.scr='downloads';" + M),
    ("M12", "m", RESET + "S.m.scr='downloads';S.dl={};" + M),
    ("M13", "m", RESET + "S.offline=true;S.m.scr='offline';" + M),
    ("M14", "m", RESET + "S.m.scr='downloads';S.m.sheetK='set35-3';S.m.sheet='del';" + M),
    ("M15", "m", RESET + "S.m.scr='account';" + M),
    ("M15B", "m", RESET + "S.m.scr='info';" + M),
    ("M15B-password", "m", RESET + "S.m.scr='account';S.m.sheet='password';" + M),
    ("M15C", "m", RESET + "S.m.scr='coach';" + M),
    ("M15D", "m", RESET + "S.m.scr='account';S.m.sheet='avatar';" + M),
    ("M15-logout", "m", RESET + "S.m.scr='account';S.m.sheet='logout';" + M),
    ("M16", "m", RESET + "S.m.scr='connect';" + M),
    ("M16-code", "m", RESET + "S.m.scr='connect';S.m.tvCode='7K4M2P';" + M),
    ("M16-linked", "m", RESET + "S.m.scr='connect';S.m.tvLinked=true;" + M),
    ("M-toast", "m", RESET + "S.m.scr='detail';S.m.toast='Đã tải xong bài 03';" + M),
    ("TV01", "tv", "initState();S.tv.scr='welcome';" + T),
    ("TV02", "tv", "initState();S.tv.scr='code';" + T),
    ("TV04", "tv", RESET + "S.tv.scr='home';" + T),
    ("TV05", "tv", RESET + "S.tv.scr='courses';" + T),
    ("TV06", "tv", RESET + "S.tv.scr='detail';" + T),
    ("TV07", "tv", RESET + "S.tv.scr='player';" + T),
    ("TV08", "tv", RESET + "S.tv.scr='player';S.tv.panel=true;" + T),
    ("TV09", "tv", RESET + "S.tv.scr='music';S.pt.kind='audio';" + T),
    ("TV10", "tv", RESET + "S.tv.scr='upnext';" + T),
    ("TV11", "tv", RESET + "S.offline=true;S.tv.scr='error';" + T),
    ("TV12", "tv", RESET + "S.tv.scr='settings';" + T),
    ("TV-toast", "tv", RESET + "S.tv.scr='home';S.tv.toast='Đã đăng nhập bằng thaovy.hlv@gmail.com';" + T),
]

FORCE_SCALE_1 = """
#phone{transform:none!important}
#tvbox{transform:none!important}
#holdM{width:409px!important;height:868px!important}
#holdT{width:1960px!important;height:1142px!important}
*{animation-duration:0s!important;animation-delay:0s!important;transition:none!important;caret-color:transparent!important}
"""

LAYOUT_JS = r"""
(rootSel) => {
  const root = document.querySelector(rootSel);
  const R = root.getBoundingClientRect();
  const pick = '[data-act],[data-go],[data-tgo],[data-tact],[data-fid],.f,.btn,.chip,.les,.card,.poster,.tabs button,.inp,.sheet,.toast,.h1,.d,.eb,.top,.sec';
  const out = [];
  const seen = new Map();
  root.querySelectorAll(pick).forEach(el => {
    const r = el.getBoundingClientRect();
    if (r.width === 0 && r.height === 0) return;
    const d = el.dataset;
    let key = d.fid || (d.act && d.act + (d.k ? ':' + d.k : d.v ? ':' + d.v : d.set ? ':' + d.set : ''))
      || (d.tact && 'tv.' + d.tact + (d.k ? ':' + d.k : d.v ? ':' + d.v : ''))
      || (d.go && 'go.' + d.go + (d.set ? ':' + d.set : '')) || (d.tgo && 'tgo.' + d.tgo + (d.set ? ':' + d.set : ''))
      || (el.tagName.toLowerCase() + '.' + [...el.classList].join('.'));
    const n = (seen.get(key) || 0) + 1; seen.set(key, n);
    if (n > 1) key += '#' + n;
    const cs = getComputedStyle(el);
    out.push({
      key,
      text: (el.innerText || '').trim().slice(0, 80),
      box: [r.left - R.left, r.top - R.top, r.width, r.height].map(v => Math.round(v * 10) / 10),
      style: {
        fontFamily: cs.fontFamily.split(',')[0].replace(/['"]/g, ''),
        fontSize: cs.fontSize, fontWeight: cs.fontWeight, letterSpacing: cs.letterSpacing,
        color: cs.color, background: cs.backgroundColor, radius: cs.borderTopLeftRadius,
        border: cs.borderTopWidth + ' ' + cs.borderTopColor, opacity: cs.opacity,
        focused: el.classList.contains('focus'),
      },
    });
  });
  return out;
}
"""


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--url", default="http://localhost:5180")
    ap.add_argument("--out", default="local/shots/prototype")
    ap.add_argument("--dpr", type=float, default=1)
    ap.add_argument("--only", default="")
    ap.add_argument("--chrome", default=os.environ.get("CHROME", ""))
    a = ap.parse_args()
    only = set(filter(None, a.only.split(",")))
    errors = []
    with sync_playwright() as p:
        kw = {"executable_path": a.chrome} if a.chrome else {}
        b = p.chromium.launch(**kw)
        for plat in ("m", "tv"):
            pg = b.new_page(viewport={"width": 2100, "height": 1300}, device_scale_factor=a.dpr)
            pg.on("pageerror", lambda e: errors.append(str(e)))
            pg.on("console", lambda m: errors.append(m.text)
                  if m.type == "error" and "Failed to load resource" not in m.text else None)
            pg.on("response", lambda r: errors.append(f"HTTP {r.status} {r.url}")
                  if r.status >= 400 and not r.url.endswith("favicon.ico") else None)
            pg.clock.install(time=1794700000000)  # 2026-11-14, frozen
            pg.goto(a.url, wait_until="networkidle")
            pg.clock.pause_at(1794700000000 + 5000)  # stop the clock: timers fire only when a state asks
            pg.add_style_tag(content=FORCE_SCALE_1)
            pg.evaluate("v => {S.view=v; layout();}", "mobile" if plat == "m" else "tv")
            pg.evaluate("document.fonts.ready")
            root = "#m" if plat == "m" else "#tv"
            d = os.path.join(a.out, "mobile" if plat == "m" else "tv")
            os.makedirs(d, exist_ok=True)
            for sid, sp, js in STATES:
                if sp != plat or (only and sid not in only):
                    continue
                pg.evaluate("js => { const v=S.view; (0,eval)(js); S.view=v; }", js)
                if plat == "tv":
                    # The focus engine marks the first .f when none is focused.
                    pg.evaluate("() => { if(!document.querySelector('#tv .f.focus')){const f=document.querySelector('#tv .f'); if(f){f.classList.add('focus'); S.tv.focus=f.dataset.fid;}} }")
                pg.locator(root).screenshot(path=os.path.join(d, sid + ".png"))
                lay = pg.evaluate(LAYOUT_JS, root)
                with open(os.path.join(d, sid + ".layout.json"), "w") as f:
                    json.dump({"id": sid, "platform": "mobile" if plat == "m" else "tv", "elements": lay},
                              f, ensure_ascii=False, indent=1)
                print(sid, len(lay), "elements")
            pg.close()
        b.close()
    if errors:
        print("PAGE ERRORS:", *errors, sep="\n  ")
        sys.exit(1)


if __name__ == "__main__":
    main()
