#!/usr/bin/env bash
# Adapted from scripts/tvos-memgate.sh (footprint sampled on the Mac, the
# device's through xctrace's Activity Monitor) and jplayer-mobile/scripts/
# m11-bg.sh at 49de3c2 (dumpsys meminfo); the launch is
# scripts/lib/target.sh's.
#
# perf-video.sh measures the memory of DanceFITVN playing a video for ten
# minutes (docs/plan/07-testing.md §5: no growth over 10 MB; local/plans/
# P8.md P8.4a): a developer's build at DFV_STATE=PERF-VIDEO10 signs in
# (the TV pairs, as TV04), plays the video of Set 35 through the system's
# engine in an A–B loop over the whole media for <minutes>, logging where
# it stands every 30 s and, with its frame log on (DFV_FRAMELOG=1), a mem
# line of the Go runtime a second (internal/app/devperf*.go, framelog.go).
# The process's memory is sampled from the Mac every DFV_PERF_EVERY
# seconds: on a simulator its phys_footprint (footprint, every 5 s), on
# Android its total PSS (dumpsys meminfo, every 30 s). The growth is the
# last sample less the first one 30 s after the play began (the warm-up).
#
#   scripts/perf-video.sh ios|android|tvos [minutes]
#
# minutes defaults to DFV_PERF_MIN, else 10. The target
# (scripts/lib/target.sh): the simulator, or DFV_IPHONE_UDID's iPhone or
# DFV_TV_UDID's Apple TV, or ANDROID_SERIAL's emulator or phone (required
# for android). On an iPhone or an Apple TV the footprint comes from
# xctrace's Activity Monitor (memory-physical-footprint, a sample a second;
# xctrace launches the app with the variables, kept as <base>.trace): no
# console is kept there, so the warm-up is the first 90 s of the trace
# (the sign-in or the pairing, then 30 s of play). On a simulator its
# accessibility runtime is off for the run and put back after.
# DFV_NOBUILD=1 installs dist/ as it is. Writes into DFV_PERF_OUT (default
# local/p8/perf): video-<target>.log (the console), video-<target>.csv
# (t_ms,bytes), video-<target>.md, video-<target>.png. Exit 1 when the
# growth is over 10 MB.
set -uo pipefail
cd "$(dirname "$0")/.."
# shellcheck source=lib/target.sh
source scripts/lib/target.sh

sys="${1:?usage: scripts/perf-video.sh ios|android|tvos [minutes]}"
minutes="${2:-${DFV_PERF_MIN:-10}}"
target_init "$sys"
out="${DFV_PERF_OUT:-local/p8/perf}"
mkdir -p "$out"
base="$out/video-$T_LABEL"
every=${DFV_PERF_EVERY:-5}
[[ $T_KIND == android ]] && every=${DFV_PERF_EVERY:-30}
sampler=""
trap '[[ -n $sampler ]] && kill $sampler 2>/dev/null; target_stop >/dev/null 2>&1 || true; target_ax_restore' EXIT
target_build "$base.build.log"
target_ax_off
target_wake
csv="$base.csv"
echo "t_ms,bytes" >"$csv"
vars=("DFV_STATE=PERF-VIDEO10" "DFV_FRAMELOG=1" "DFV_PERF_MIN=$minutes")

if [[ $T_KIND == device ]]; then
	rm -rf "$base.trace"
	: >"$base.log"
	xargs=()
	for kv in "${vars[@]}"; do xargs+=(--env "$kv"); done
	xcrun xctrace record --template 'Activity Monitor' --device "$T_ID" --time-limit "$((minutes + 3))m" \
		--output "$base.trace" --no-prompt "${xargs[@]}" --launch -- "$T_BUNDLE" >"$base.xctrace.txt" 2>&1 &
	rec=$!
	n=0
	until grep -q "Ctrl-C to stop" "$base.xctrace.txt" 2>/dev/null; do
		((n++ < 240)) || { echo "xctrace did not start: $base.xctrace.txt" >&2; exit 1; }
		kill -0 "$rec" 2>/dev/null || { echo "xctrace ended: $base.xctrace.txt" >&2; exit 1; }
		sleep 0.5
	done
	echo "perf-video: $T_LABEL: recording $((minutes * 60 + 90)) s"
	sleep $((minutes * 60 + 90))
	kill -INT "$rec" 2>/dev/null || true
	wait "$rec" 2>/dev/null || true
	target_shot "$base.png"
	xcrun xctrace export --input "$base.trace" --toc >"$base.toc.xml"
	xcrun xctrace export --input "$base.trace" \
		--xpath '/trace-toc/run[@number="1"]/data/table[@schema="activity-monitor-process-live"]' >"$base.live.xml"
	python3 -I scripts/lib/xctrace_footprint.py "$base.toc.xml" "$base.live.xml" DanceFITVN >>"$csv"
else
	target_launch "$base.log" "${vars[@]}"
	target_wait "$base.log" "dfv: perf video start t=" 120 || { echo "the video did not start: $base.log" >&2; exit 1; }
	echo "perf-video: $T_LABEL: playing $minutes min, a sample every $every s"
	(
		pid=""
		while :; do
			[[ -n $pid ]] && kill -0 "$pid" 2>/dev/null || pid=$(target_pid)
			if [[ -n $pid ]]; then
				b=$(target_mem_bytes "$pid")
				[[ -n $b ]] && echo "$(t_now_ms),$b" >>"$csv"
			fi
			sleep "$every"
		done
	) &
	sampler=$!
	target_wait "$base.log" "dfv: perf video end t=" $((minutes * 60 + 60)) || echo "perf-video: no end mark (see $base.log)" >&2
	kill "$sampler" 2>/dev/null
	sampler=""
	sleep 1.5 # the last mem line
	target_shot "$base.png"
	target_stop
fi

python3 -I - "$base" "$T_LABEL" "$minutes" "$T_KIND" "$T_AX" <<'PY'
import csv, re, statistics, sys
base, label, minutes, kind, ax = sys.argv[1], sys.argv[2], int(sys.argv[3]), sys.argv[4], sys.argv[5]
WARM = 30_000
start = end = None
heap, at = [], []
for raw in open(base + ".log", errors="replace"):
    line = raw.strip().replace("\r", "")
    m = re.search(r"dfv: perf video (start|at|end) t=(\d+) pos=([\d.]+) dur=([\d.]+) state=(\d+) loop=(\w+)(?: tick=([\d.]+))?", line)
    if m:
        t = int(m.group(2))
        if m.group(1) == "start":
            start = t
        elif m.group(1) == "end":
            end = t
        at.append((t, float(m.group(3)), float(m.group(4)), m.group(5), m.group(6), m.group(7) or "-"))
    m = re.search(r"^mem t=(\d+) goroutines=(\d+) heap_live=(\d+) mapped=(\d+)", line)
    if m:
        heap.append((int(m.group(1)), int(m.group(2)), int(m.group(3)), int(m.group(4))))
samples = [(int(r["t_ms"]), int(r["bytes"])) for r in csv.DictReader(open(base + ".csv"))]
if kind == "device":
    warm_from = samples[0][0] + 60_000 + WARM if samples else 0  # no console: the sign-in or pairing, then 30 s of play
else:
    warm_from = (start or 0) + WARM
sel = [s for s in samples if s[0] >= warm_from and (end is None or s[0] <= end + 2_000)]
mb = lambda v: "-" if v is None else f"{v / 1e6:.1f}"
first = sel[0][1] if sel else None
last = sel[-1][1] if sel else None
growth = None if first is None else last - first
hsel = [h for h in heap if h[0] >= warm_from]
span = (sel[-1][0] - sel[0][0]) / 60000 if len(sel) > 1 else 0
ok = growth is not None and growth <= 10e6 and span >= minutes - 1.5
what = {"sim": "phys_footprint (footprint)", "android": "total PSS (dumpsys meminfo)", "device": "phys_footprint (xctrace Activity Monitor)"}[kind]
lines = [f"# Memory of a {minutes}-minute video, {label}", ""]
if ax == "off":
    lines += ["The simulator's accessibility runtime off for the run (put back after).", ""]
lines += [f"Memory: {what}; {len(samples)} samples, {len(sel)} after the warm-up ({WARM // 1000} s of play) over {span:.1f} min.", "",
          "| | first | last | growth | peak |", "|---|---|---|---|---|",
          f"| process (MB) | {mb(first)} | {mb(last)} | {mb(growth)} | {mb(max((s[1] for s in sel), default=None))} |"]
if hsel:
    lines.append(f"| Go heap live (MB) | {mb(hsel[0][2])} | {mb(hsel[-1][2])} | {mb(hsel[-1][2] - hsel[0][2])} | {mb(max(h[2] for h in hsel))} |")
    lines.append(f"| Go mapped (MB) | {mb(hsel[0][3])} | {mb(hsel[-1][3])} | {mb(hsel[-1][3] - hsel[0][3])} | {mb(max(h[3] for h in hsel))} |")
    lines.append(f"| goroutines | {hsel[0][1]} | {hsel[-1][1]} | {hsel[-1][1] - hsel[0][1]} | {max(h[1] for h in hsel)} |")
if at:
    lines += ["", "The player every 30 s (pos: the engine's reading, Player.SyncPos; tick: what the last time event",
              "told, the screen's; dur in s; state 3 = playing, internal/player.State):", "",
              "| t (s) | pos | tick | dur | state | loop |", "|---|---|---|---|---|---|"]
    lines += [f"| {(t - at[0][0]) / 1000:.0f} | {p:.1f} | {tk} | {d:.1f} | {s} | {lo} |" for t, p, d, s, lo, tk in at]
lines += ["", f"Gate (07 §5): growth ≤ 10 MB over the {minutes} minutes: {'PASS' if ok else 'FAIL'}"
          + (" (a simulator or an emulator: indicative, R-P8-5)" if "sim" in label or "emulator" in label else "")
          + ("" if span >= minutes - 1.5 else f" — only {span:.1f} min sampled")]
open(base + ".md", "w").write("\n".join(lines) + "\n")
print("\n".join(lines))
sys.exit(0 if ok else 1)
PY
