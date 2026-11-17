#!/usr/bin/env bash
# tvos-framestats.sh times the frames of focus moves on TV05 (the course
# grid, 15 posters) on the Apple TV 4K simulator (local/plans/P7.md P7.5c,
# 07 §5 "TV di focus ≤ 1 khung"): the developer's app at DFV_STATE=TV05 with
# its frame log (DFV_FRAMELOG=1, GDESK_FRAMESTATS=1, internal/app/
# framelog.go), driven by the Siri Remote (scripts/tvos-remote.sh) along a
# snake through the grid and back (the grid scrolls), then the frames from
# the first press to 1 s after the last are summed up: p50, p95, max of
# their totals (layout + paint + present on the UI thread), the frames over
# 16.7 ms. The gate: p95 ≤ 16.7 ms.
#
#   scripts/tvos-framestats.sh [tag]
#
# On the paired Apple TV: DFV_TV_UDID=<UDID> (with DFV_TEAM) runs the same steps through
# scripts/tvos-remote-device.sh (default output local/p8/appletv/framestats).
#
# Environment (all optional): DFV_TVSIM (default "DanceFITVN Apple TV"),
# DFV_NOBUILD=1, DFV_FS_ROUNDS (snakes down and up, default 2), DFV_FS_GAP
# (ms after each press, default 500), DFV_FS_OUT (default
# local/p7/p75c/part2). Outputs in DFV_FS_OUT: <tag>.log (the console, with
# the frame log), <tag>.xcodebuild.log, <tag>-*.png, <tag>.md. Exit 1 when
# the gate fails.
set -euo pipefail
root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root"

tag=${1:-framestats}
remote=scripts/tvos-remote.sh out_default=local/p7/p75c/part2
if [[ -n ${DFV_TV_UDID:-} ]]; then
	remote=scripts/tvos-remote-device.sh out_default=local/p8/appletv/framestats
fi
out=${DFV_FS_OUT:-$out_default}
rounds=${DFV_FS_ROUNDS:-2}
gap=${DFV_FS_GAP:-500}
mkdir -p "$out"

# One snake: down into the grid, across each of its five rows, back up to
# the filters.
snake=""
for row in 1 2 3 4 5; do
	if ((row % 2)); then dir=right; else dir=left; fi
	snake+="down@$gap $dir@$gap $dir@$gap "
done
for _ in 1 2 3 4 5; do snake+="up@$gap "; done
steps="wait:1500 shot:start mark:moves"
for ((i = 0; i < rounds; i++)); do steps+=" $snake"; done
steps+=" wait:1000 mark:end shot:end"

DFV_STATE=TV05 DFV_FRAMELOG=1 GDESK_FRAMESTATS=1 DFV_FOCUSTRACE=1 DFV_REMOTE_OUT="$out" \
	"$remote" "$steps" "$tag" >"$out/$tag.remote.txt" 2>&1 || { echo "the remote run failed: $out/$tag.remote.txt" >&2; }

python3 -I - "$out/$tag" <<'EOF'
import re, sys
base = sys.argv[1]
marks = {}
for line in open(base + ".xcodebuild.log", errors="replace"):
    m = re.search(r"dfvremote: (\d+\.\d+) mark (\S+)", line)
    if m:
        marks[m.group(2)] = int(float(m.group(1)) * 1000)
frames, keys, header = [], [], ""
for line in open(base + ".log", errors="replace"):
    line = line.strip()
    if line.startswith("# dfv-framelog"):
        header = line
    f = line.split()
    if len(f) == 10 and f[0] == "fs":
        frames.append((int(f[1]), [int(x) for x in f[3:7]], f[7] == "true", f[8] == "true"))
    elif len(f) == 4 and f[0] == "key" and f[2] == "down":
        keys.append(int(f[3][2:]))
t0 = min(keys) if keys else marks.get("moves", 0)
t1 = (max(keys) if keys else marks.get("end", 0)) + 1000
sel = [fr for fr in frames if t0 <= fr[0] <= t1]
def pct(xs, p):
    xs = sorted(xs)
    return xs[min(len(xs) - 1, (len(xs) * p) // 100)] if xs else 0
tot = [fr[1][3] / 1000 for fr in sel]
chg = [fr[1][3] / 1000 for fr in sel if not fr[3]]
ok = bool(tot) and pct(tot, 95) <= 16.7
lines = [header, "",
         f"Presses: {len(keys)}; frames from the first press to 1 s after the last: {len(sel)} ({len(chg)} changed, {sum(1 for f in sel if f[2])} on the GPU).", "",
         "| frames | n | p50 (ms) | p95 (ms) | max (ms) | > 16.7 ms | > 33.4 ms |", "|---|---|---|---|---|---|---|"]
for name, xs in (("all", tot), ("changed", chg)):
    lines.append(f"| {name} | {len(xs)} | {pct(xs, 50):.2f} | {pct(xs, 95):.2f} | {max(xs, default=0):.2f} | {sum(1 for x in xs if x > 16.7)} | {sum(1 for x in xs if x > 33.4)} |")
parts = ("layout", "paint", "present")
lines += ["", "| part | p50 (ms) | p95 (ms) | max (ms) |", "|---|---|---|---|"]
for i, name in enumerate(parts):
    xs = [fr[1][i] / 1000 for fr in sel]
    lines.append(f"| {name} | {pct(xs, 50):.2f} | {pct(xs, 95):.2f} | {max(xs, default=0):.2f} |")
# By the press each frame follows: Up/Down change the grid's row (it
# scrolls), Left/Right move along a row.
evs = sorted([(fr[0], "f", fr[1][3] / 1000) for fr in sel] + [(k, "k", None) for k in keys])
names = {}
for line in open(base + ".log", errors="replace"):
    f = line.split()
    if len(f) == 4 and f[0] == "key" and f[2] == "down":
        names[int(f[3][2:])] = f[1]
groups, cur = {"Up/Down (the grid scrolls)": [], "Left/Right (along a row)": []}, None
for t, kind, v in evs:
    if kind == "k":
        cur = "Up/Down (the grid scrolls)" if names.get(t) in ("Up", "Down") else "Left/Right (along a row)"
    elif cur:
        groups[cur].append(v)
lines += ["", "| frames after | n | p50 (ms) | p95 (ms) | max (ms) | > 16.7 ms |", "|---|---|---|---|---|---|"]
for name, xs in groups.items():
    lines.append(f"| {name} | {len(xs)} | {pct(xs, 50):.2f} | {pct(xs, 95):.2f} | {max(xs, default=0):.2f} | {sum(1 for x in xs if x > 16.7)} |")
lines += ["", f"Gate p95 ≤ 16.7 ms: {'PASS' if ok else 'FAIL'}"]
open(base + ".md", "w").write("\n".join(lines) + "\n")
print("\n".join(lines))
sys.exit(0 if ok else 1)
EOF
