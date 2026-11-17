#!/usr/bin/env bash
# Adapted from jplayer-tv/scripts/tvos-soak.sh at 781bee5 (app_pid_of,
# footprint_bytes; the run, trimmed to one pass of the remote).
#
# tvos-memgate.sh runs the memory gate R1 of local/plans/P7.md P7.5c on the
# Apple TV 4K simulator: the developer's app from TV01-confirm (TV01, the
# mock's pairing confirmed at the first poll) driven by the Siri Remote
# (scripts/tvos-remote.sh), its frame log on (DFV_FRAMELOG=1,
# GDESK_FRAMESTATS=1: a mem line of the Go runtime a second), and the
# process's phys_footprint sampled on the Mac every second. Three phases,
# each after one warm-up round, each between two marks:
#
#   A  TV01: Select on w2, 20 toasts (3 s apart: each shows 2.4 s and goes)
#   B  w1 → TV02 → paired → TV04 → Courses (TV05): 10 trips TV05 → TV06 → Menu
#   C  TV06 → TV07 (the lesson, paused) → p2: 20 times panel open, Menu
#
# The gate: the growth over the phases, (A1-A0)+(B1-B0)+(C1-C0) of the
# footprint at the marks (the median of the samples in the 2 s before each),
# is at most 10 MB (07 §5).
#
#   scripts/tvos-memgate.sh [tag]
#
# On the paired Apple TV: DFV_TV_UDID=<UDID> (with DFV_TEAM) drives the app through
# scripts/tvos-remote-device.sh and takes the footprint from xctrace's
# Activity Monitor (memory-physical-footprint of the process DanceFITVN,
# one sample a second; xctrace launches the app, kept as <tag>.trace; adapted from jplayer-tv/scripts/tvos-soak.sh at 781bee5,
# its device samples), default output local/p8/appletv/memgate. The marks
# are the Apple TV's clock, the trace's the Mac's: both keep network time.
#
# Environment (all optional): DFV_TVSIM (default "DanceFITVN Apple TV", the
# 4K one), DFV_NOBUILD=1 (install dist/ as it is), DFV_MEM_TOASTS (20),
# DFV_MEM_TRIPS (10), DFV_MEM_PANELS (20), DFV_MEM_OUT (default
# local/p7/p75c/part2). Outputs in DFV_MEM_OUT: <tag>.log (the console, with
# the frame log), <tag>.xcodebuild.log (the marks), <tag>.footprint.csv,
# <tag>-*.png, <tag>.md (the table). Exit 1 when the gate fails.
set -euo pipefail
root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root"

tag=${1:-memgate}
device=${DFV_TVSIM:-DanceFITVN Apple TV}
on_device=${DFV_TV_UDID:-}
if [[ -n $on_device ]]; then
	out=${DFV_MEM_OUT:-local/p8/appletv/memgate}
else
	out=${DFV_MEM_OUT:-local/p7/p75c/part2}
fi
toasts=${DFV_MEM_TOASTS:-20}
trips=${DFV_MEM_TRIPS:-10}
panels=${DFV_MEM_PANELS:-20}
mkdir -p "$out"
udid=$(xcrun simctl list devices | grep -F "$device (" | head -1 | grep -oE '[0-9A-F]{8}-[0-9A-F-]{27}' || true)
udid=${udid:-$device}

# app_pid_of prints the app's process on the Mac: its executable in the
# device's bundle folder.
app_pid_of() { pgrep -f "Devices/$udid/data/Containers/Bundle/Application/.*/DanceFITVN\.app/DanceFITVN" | head -1 || true; }

# footprint_bytes prints the phys_footprint of the process $1 in bytes, or
# nothing when footprint cannot read it.
footprint_bytes() {
	footprint -p "$1" -f bytes --noCategories 2>/dev/null |
		awk '/[Ff]ootprint:/ { for (i = 1; i <= NF; i++) if ($i ~ /^[0-9]+$/) { print $i; exit } }' || true
}

# rep prints $2 $1 times, space separated.
rep() { local i; for ((i = 0; i < $1; i++)); do printf '%s ' "$2"; done; }

toast="select wait:2400"
trip="down select@2000 menu@2000" # Menu back to TV05 puts the focus on cf0 (the default rule): Down is a card
panel="select@1500 menu@1500"
steps="wait:2000 shot:tv01 right $toast mark:A0 $(rep "$toasts" "$toast") wait:1000 mark:A1 shot:a1"
steps+=" left select wait:7000 shot:tv04 up right select@2000 down shot:tv05 $trip mark:B0 $(rep "$trips" "$trip") wait:1000 mark:B1 shot:b1"
steps+=" down select@2000 shot:tv06 select@3000 playpause@1500 right shot:tv07 $panel mark:C0 $(rep "$panels" "$panel") wait:1000 mark:C1 shot:c1"

csv=$out/$tag.footprint.csv
echo "t_ms,footprint_b" >"$csv"
if [[ -n $on_device ]]; then
	# xctrace launches the app itself, with the variables the simulator's
	# run gives it: recording every process ended after a second over
	# Wi-Fi ("Device got disconnected"), and --attach finds no app that
	# devicectl launched (by name, bundle or process id). Its console is
	# then not kept (no Go heap column); the steps start 6 s after the
	# launch (the app is ready in about 2 s) and drive the app running.
	rm -rf "$out/$tag.trace" "$out/$tag.log" "$out/$tag.xctrace.txt"
	app=dist/tvos-arm64/DanceFITVN.app
	if [[ -z ${DFV_NOBUILD:-} ]]; then
		scripts/dev-build.sh tvos -device >"$out/$tag.build.log" 2>&1 || { echo "the app did not build: $out/$tag.build.log" >&2; exit 1; }
		scripts/tvos-device-sign.sh "$app" >>"$out/$tag.build.log"
	fi
	xcrun devicectl device install app --device "$on_device" "$app" >>"$out/$tag.build.log" 2>&1 ||
		{ echo "install failed: $out/$tag.build.log" >&2; exit 1; }
	xcrun xctrace record --template 'Activity Monitor' --device "$on_device" --time-limit 30m \
		--output "$out/$tag.trace" --no-prompt --env DFV_STATE=TV01-confirm --env DFV_FRAMELOG=1 \
		--env GDESK_FRAMESTATS=1 --env DFV_FOCUSTRACE=1 --launch -- com.jivecode.dancefitvn >"$out/$tag.xctrace.txt" 2>&1 &
	sampler=$!
	trap 'kill -INT $sampler 2>/dev/null || true' EXIT
	n=0
	until grep -q "Ctrl-C to stop" "$out/$tag.xctrace.txt" 2>/dev/null; do
		((n++ < 240)) || { echo "xctrace did not start: $out/$tag.xctrace.txt" >&2; exit 1; }
		kill -0 "$sampler" 2>/dev/null || { echo "xctrace ended: $out/$tag.xctrace.txt" >&2; exit 1; }
		sleep 0.5
	done
	sleep 6
	DFV_NOLAUNCH=1 DFV_REMOTE_OUT="$out" scripts/tvos-remote-device.sh "$steps" "$tag" >"$out/$tag.remote.txt" 2>&1 ||
		{ echo "the remote run failed: $out/$tag.remote.txt" >&2; }
	[[ -f $out/$tag.log ]] || : >"$out/$tag.log" # no console: the analyzer's heap columns stay empty
	kill -INT "$sampler" 2>/dev/null || true
	wait "$sampler" 2>/dev/null || true
	xcrun xctrace export --input "$out/$tag.trace" --toc >"$out/$tag.toc.xml"
	xcrun xctrace export --input "$out/$tag.trace" \
		--xpath '/trace-toc/run[@number="1"]/data/table[@schema="activity-monitor-process-live"]' >"$out/$tag.live.xml"
	python3 -I - "$out/$tag.toc.xml" "$out/$tag.live.xml" DanceFITVN >>"$csv" <<'PY'
import datetime, re, sys
import xml.etree.ElementTree as ET
toc, live, name = sys.argv[1:4]
start = re.search(r"<start-date>([^<]+)</start-date>", open(toc).read()).group(1)
t0 = datetime.datetime.fromisoformat(start).timestamp() * 1000
byid = {}
def val(el):  # an element or the one its ref names
    r = el.get("ref")
    return byid[r] if r else el
def note(el):
    if el.get("id"):
        byid[el.get("id")] = el
    for c in el:
        note(c)
root = ET.parse(live).getroot()
cols = [c.findtext("mnemonic") for c in root.iter("col")]
i_start, i_proc, i_fp = cols.index("start"), cols.index("process"), cols.index("memory-physical-footprint")
for row in root.iter("row"):
    note(row)
    cells = list(row)
    if len(cells) <= i_fp:
        continue
    proc = val(cells[i_proc])
    if not (proc.get("fmt") or "").startswith(name + " "):
        continue
    t = int(val(cells[i_start]).text or 0)
    fp = val(cells[i_fp]).text
    if fp:
        print(f"{int(t0 + t / 1e6)},{int(fp)}")
PY
else
(
	pid=""
	while :; do
		[[ -n $pid ]] && kill -0 "$pid" 2>/dev/null || pid=$(app_pid_of)
		if [[ -n $pid ]]; then
			fp=$(footprint_bytes "$pid")
			[[ -n $fp ]] && echo "$(python3 -I -c 'import time; print(int(time.time()*1000))'),$fp" >>"$csv"
		fi
		sleep 1
	done
) &
sampler=$!
trap 'kill $sampler 2>/dev/null || true' EXIT

DFV_TVSIM="$udid" DFV_REMOTE_OUT="$out" DFV_STATE=TV01-confirm DFV_FRAMELOG=1 GDESK_FRAMESTATS=1 DFV_FOCUSTRACE=1 \
	scripts/tvos-remote.sh "$steps" "$tag" >"$out/$tag.remote.txt" 2>&1 || { echo "the remote run failed: $out/$tag.remote.txt" >&2; }
kill "$sampler" 2>/dev/null || true
fi

python3 -I - "$out/$tag" "$toasts" "$trips" "$panels" <<'EOF'
import csv, re, statistics, sys
base = sys.argv[1]
marks = {}
for line in open(base + ".xcodebuild.log", errors="replace"):
    m = re.search(r"dfvremote: (\d+\.\d+) mark (\S+)", line)
    if m:
        marks[m.group(2)] = int(float(m.group(1)) * 1000)
fp = [(int(r["t_ms"]), int(r["footprint_b"])) for r in csv.DictReader(open(base + ".footprint.csv"))]
heap = []
for line in open(base + ".log", errors="replace"):
    m = re.search(r"^mem t=(\d+) goroutines=(\d+) heap_live=(\d+) mapped=(\d+)", line.strip())
    if m:
        heap.append((int(m.group(1)), int(m.group(3)), int(m.group(4))))
def at(series, t, col=1):
    xs = [s[col] for s in series if t - 2000 <= s[0] <= t]
    return statistics.median(xs) if xs else None
mb = lambda v: "-" if v is None else f"{v / 1e6:.1f}"
rows, growth, ok = [], 0.0, True
nt, ntr, np = sys.argv[2:5]
for p, what in (("A", f"{nt} toasts on TV01"), ("B", f"{ntr} trips TV05-TV06"), ("C", f"{np} panel open/close on TV07")):
    a, b = marks.get(p + "0"), marks.get(p + "1")
    if a is None or b is None:
        rows.append(f"| {p} | {what} | - | - | - | - | - | marks missing |")
        ok = False
        continue
    f0, f1 = at(fp, a), at(fp, b)
    h0, h1 = at(heap, a), at(heap, b)
    m0, m1 = at(heap, a, 2), at(heap, b, 2)
    d = None if f0 is None or f1 is None else f1 - f0
    if d is None:
        ok = False
    else:
        growth += d
    rows.append(f"| {p} | {what} | {mb(f0)} | {mb(f1)} | {mb(d)} | {mb(h0)} → {mb(h1)} | {mb(m0)} → {mb(m1)} | {(b - a) / 1000:.0f} s |")
peak = max((v for _, v in fp), default=0)
first = fp[0][1] if fp else 0
last = fp[-1][1] if fp else 0
ok = ok and growth <= 10e6
lines = ["| phase | what | footprint at start (MB) | at end (MB) | growth (MB) | Go heap live (MB) | Go mapped (MB) | span |",
         "|---|---|---|---|---|---|---|---|"] + rows
lines += ["", f"Growth over the phases: {growth / 1e6:.1f} MB (gate ≤ 10 MB): {'PASS' if ok else 'FAIL'}",
          f"Footprint first sample {mb(first)} MB, peak {mb(peak)} MB, last {mb(last)} MB; {len(fp)} samples, {len(heap)} mem lines."]
open(base + ".md", "w").write("\n".join(lines) + "\n")
print("\n".join(lines))
sys.exit(0 if ok else 1)
EOF
