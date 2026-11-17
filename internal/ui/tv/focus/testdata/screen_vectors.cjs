// Adapted from jplayer-tv/internal/ui/focus/testdata/pick_vectors.cjs at 781bee5.
// Prints focus-move vectors from the HTML prototype for TestScreenVectors.
// Usage (from the repo root):
//   node internal/ui/tv/focus/testdata/screen_vectors.cjs \
//     reference/prototype/index.html local/shots/prototype-kit/tv \
//     > internal/ui/tv/focus/testdata/screen_vectors.json
// The prototype's own moveFocus (index.html) runs in a vm over a DOM stub whose
// .f elements are the focusable boxes of TV01, TV02, TV04..TV12 layout.json.
// Rules (local/plans/P7.md section P7.6 step 2):
//   - the focused box in the capture is scaled; it is un-scaled by its transform;
//   - each element in turn is the focused one with its rectangle scaled as the
//     prototype does (nav 1.04, trow 1.02, topt 1, the rest 1.06), then four moves;
//   - TV02 drops c1 (K2, the simulated confirm button);
//   - TV08 has its panel open: the candidates are the panel's o*, ot*, oc.
// The output rectangles are the unscaled ones, in document order, with the
// prototype's answer per (from, dir); "to" equals "from" when the focus stays.
const fs = require('fs'), vm = require('vm'), path = require('path');
const proto = path.resolve(process.argv[2] || path.join('reference', 'prototype', 'index.html'));
const shots = path.resolve(process.argv[3] || path.join('local', 'shots', 'prototype-kit', 'tv'));
const SCREENS = ['TV01', 'TV02', 'TV04', 'TV05', 'TV06', 'TV07', 'TV08', 'TV09', 'TV10', 'TV11', 'TV12'];
const DIRS = ['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown'];

// moveFocusSource returns the source text of the prototype's moveFocus.
function moveFocusSource() {
  const src = fs.readFileSync(proto, 'utf8');
  const start = src.indexOf('function moveFocus(dir){');
  if (start < 0) throw new Error('moveFocus not found');
  let depth = 0;
  for (let i = src.indexOf('{', start); i < src.length; i++) {
    if (src[i] === '{') depth++;
    else if (src[i] === '}' && --depth === 0) return src.slice(start, i + 1);
  }
  throw new Error('moveFocus is not closed');
}

// focusScale is the scale the prototype's CSS gives the focused element of a fid.
const focusScale = (fid) => (fid.startsWith('nav') ? 1.04 : /^(dl|pn|s)\d/.test(fid) ? 1.02 : /^(o|ot|oc)\d*$/.test(fid) ? 1 : 1.06);
const scaleOf = (t) => {
  const m = /^matrix\(([-\d.e]+),/.exec(t || '');
  return m ? +m[1] : 1;
};
const scaled = (r, s) => ({ x: r.x + (r.w - r.w * s) / 2, y: r.y + (r.h - r.h * s) / 2, w: r.w * s, h: r.h * s });
const round = (v) => Math.round(v * 1000) / 1000;

// items lists the .f boxes of a layout, unscaled, in document order.
function items(layout) {
  const out = [];
  for (const e of layout.elements) {
    if (!/(^|\.)f(\.|$)/.test(e.path) || /(^|\.)f(\.|$)/.test(e.parent)) continue;
    const s = scaleOf(e.style && e.style.transform);
    const r = s === 1 ? { x: e.x, y: e.y, w: e.w, h: e.h } : scaled(e, 1 / s);
    out.push({ fid: e.key, x: round(r.x), y: round(r.y), w: round(r.w), h: round(r.h) });
  }
  return out;
}

function run(screen) {
  const layout = JSON.parse(fs.readFileSync(path.join(shots, screen + '.layout.json'), 'utf8'));
  let rects = items(layout);
  if (screen === 'TV02') rects = rects.filter((r) => r.fid !== 'c1');
  const panel = screen === 'TV08';
  const cands = panel ? rects.filter((r) => /^o/.test(r.fid)) : rects;
  const S = { tv: { panel, focus: null } };
  let curFid = null;
  const els = cands.map((r) => ({
    dataset: { fid: r.fid },
    classList: {
      add() {}, remove() {},
      contains: (c) => c === 'focus' && r.fid === curFid,
    },
    getBoundingClientRect() {
      const q = r.fid === curFid ? scaled(r, focusScale(r.fid)) : r;
      return { left: q.x, top: q.y, width: q.w, height: q.h, right: q.x + q.w, bottom: q.y + q.h };
    },
  }));
  const $tv = {
    querySelectorAll: () => els,
    querySelector: () => ({ contains: () => true }),
  };
  const ctx = vm.createContext({ S, $tv, scrollTV() {}, renderTV() {} });
  vm.runInContext(moveFocusSource() + '\nthis.moveFocus = moveFocus;', ctx, { filename: 'index.html' });
  const cases = [];
  for (const r of cands) {
    for (const dir of DIRS) {
      curFid = r.fid;
      S.tv.focus = r.fid;
      ctx.moveFocus(dir);
      cases.push({ from: r.fid, dir, to: S.tv.focus });
    }
  }
  return { screen, rects: cands, cases };
}

const screens = SCREENS.map(run);
process.stdout.write(JSON.stringify({ screens }, null, 1) + '\n');
console.error('screen vectors: ' + screens.map((s) => `${s.screen} ${s.rects.length}/${s.cases.length}`).join(', '));
