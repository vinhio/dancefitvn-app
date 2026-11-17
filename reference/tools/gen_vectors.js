#!/usr/bin/env node
// Emits test vectors for the Go tests from the prototype's own code.
//
// It loads reference/prototype/index.html, takes the pure helpers out of
// its <script> (sec, fmt, mb, the TV focus move), runs them in a vm with
// stub DOM objects, and writes JSON the Go tests read:
//
//   fmt.json     seconds -> "mm:ss"                       (fmt)
//   size.json    bytes -> "8,8 MB" (prototype mb(): dur*0.04 MB, i.e. bytes/1e6, 1 decimal, comma)
//   focus.json   seeded rectangle layouts + arrow -> id the focus moves to (moveFocus)
//
// Usage: node reference/tools/gen_vectors.js [outDir]   (default local/vectors)
'use strict';
const fs = require('fs');
const path = require('path');
const vm = require('vm');

const root = path.resolve(__dirname, '..', 'prototype', 'index.html');
const html = fs.readFileSync(root, 'utf8');
const js = html.slice(html.lastIndexOf('<script>') + 8, html.lastIndexOf('</script>'));

function grab(re, name) {
  const m = js.match(re);
  if (!m) throw new Error('prototype changed: cannot find ' + name);
  return m[0];
}
const src = [
  grab(/const sec=s=>[^\n]*\n/, 'sec'),
  grab(/const fmt=t=>[^\n]*\n/, 'fmt'),
  grab(/function moveFocus\(dir\)\{[\s\S]*?\n\}\n/, 'moveFocus'),
].join('\n');

// Seeded LCG, same constants as the prototype's qrSvg.
let seed = 20261114;
const rnd = () => ((seed = (seed * 1103515245 + 12345) & 0x7fffffff) / 0x7fffffff);

function el(id, x, y, w, h) {
  const cls = new Set(['f']);
  return {
    id, dataset: { fid: id },
    classList: { contains: c => cls.has(c), add: c => cls.add(c), remove: c => cls.delete(c) },
    getBoundingClientRect: () => ({ left: x, top: y, right: x + w, bottom: y + h, width: w, height: h }),
  };
}

const sandbox = { S: { tv: { panel: false, focus: null } }, renderTV() {}, scrollTV() {}, $tv: null, Math, String };
vm.createContext(sandbox);
vm.runInContext(src + '\nthis.fmt=fmt;this.sec=sec;this.moveFocus=moveFocus;', sandbox);

const out = process.argv[2] || 'local/vectors';
fs.mkdirSync(out, { recursive: true });

// fmt
const secs = [0, 0.4, 1, 9, 59, 60, 61, 72, 148, 220, 320, 599, 600, 3599, 3600, 5999, -3];
fs.writeFileSync(path.join(out, 'fmt.json'), JSON.stringify(secs.map(s => ({ sec: s, want: sandbox.fmt(s) })), null, 1));

// size: prototype mb(l) = (l.dur*0.04).toFixed(1).replace('.',',')+' MB'; the API gives bytes = dur*40000.
const durs = [180, 185, 220, 262, 299, 320, 205, 240, 280, 1];
fs.writeFileSync(path.join(out, 'size.json'), JSON.stringify(durs.map(d => ({
  bytes: d * 40000, want: (d * 0.04).toFixed(1).replace('.', ',') + ' MB',
})), null, 1));

// focus
const cases = [];
const dirs = ['ArrowRight', 'ArrowLeft', 'ArrowDown', 'ArrowUp'];
for (let c = 0; c < 400; c++) {
  const n = 3 + Math.floor(rnd() * 9);
  const items = [];
  // Mix grid-like rows with random boxes, as TV screens have rows of cards and side lists.
  for (let i = 0; i < n; i++) {
    const grid = rnd() < 0.6;
    const x = grid ? 80 + (i % 4) * 430 : Math.round(rnd() * 1700);
    const y = grid ? 120 + Math.floor(i / 4) * 300 : Math.round(rnd() * 900);
    const w = grid ? 400 : 120 + Math.round(rnd() * 400);
    const h = grid ? 260 : 60 + Math.round(rnd() * 200);
    items.push(el('f' + i, x, y, w, h));
  }
  const start = Math.floor(rnd() * n);
  const dir = dirs[Math.floor(rnd() * 4)];
  items.forEach((it, i) => i === start ? it.classList.add('focus') : it.classList.remove('focus'));
  sandbox.$tv = { querySelectorAll: () => items, querySelector: () => null };
  sandbox.S.tv.focus = 'f' + start;
  sandbox.moveFocus(dir);
  cases.push({
    rects: items.map(it => { const r = it.getBoundingClientRect(); return { id: it.id, x: r.left, y: r.top, w: r.width, h: r.height }; }),
    start: 'f' + start, dir, want: sandbox.S.tv.focus,
  });
}
fs.writeFileSync(path.join(out, 'focus.json'), JSON.stringify(cases));
console.log('wrote', out, ': fmt', secs.length, 'size', durs.length, 'focus', cases.length);
