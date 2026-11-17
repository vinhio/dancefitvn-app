#!/usr/bin/env bash
# New for P8.4a (local/plans/P8.md): jPlayer has no launch measure; the
# launch, wait and stop are scripts/lib/target.sh's (from shot.sh,
# tvos-remote-device.sh and jplayer-mobile/scripts/m11-bg.sh at 49de3c2).
#
# perf-start.sh times the launch of DanceFITVN to its first Home frame
# (docs/plan/07-testing.md §5: ≤ 1.5 s on an iPhone 13, ≤ 2.5 s on a
# mid-range Android; the Apple TV's TV04 is held to the iPhone's): a
# developer's build signs in once (DFV_STATE=M03, TV04 on the TV: the
# mock's session kept in the vault, Q-P8-17), then is launched cold N
# times with no variable; each launch logs "dancefitvn: mark home <ms>
# t=<unix ms> from=process" after the first frame showing Home with its
# data (internal/app/devperf.go), ms counted from the system's start of
# the process (sysctl kern.proc on iOS and tvOS, /proc on Android). The
# splash (600 ms at least, 03 §1) is inside.
#
#   scripts/perf-start.sh ios|android|tvos [N]
#
# N defaults to DFV_PERF_N, else 10. The target (scripts/lib/target.sh):
# the simulator, or DFV_IPHONE_UDID / DFV_TV_UDID's device, or
# ANDROID_SERIAL's emulator or phone (required for android). Also
# reported: the launch command to the mark by the Mac's clock (simulators
# and the emulator share it; a device's runs on its own) and Android's
# am start -W TotalTime (to the first frame of the activity: the splash).
# On a simulator its accessibility runtime is turned off for the run and
# put back after (target_ax_off: loaded, it adds about 2 s to a launch).
# DFV_NOBUILD=1 installs dist/ as it is. Writes into DFV_PERF_OUT (default
# local/p8/perf): start-<target>.md (the table, median and p90),
# start-<target>-<i>.log (each launch's console). Exit 1 when the median
# misses the gate, 2 on a launch with no mark.
set -uo pipefail
cd "$(dirname "$0")/.."
# shellcheck source=lib/target.sh
source scripts/lib/target.sh

sys="${1:?usage: scripts/perf-start.sh ios|android|tvos [N]}"
n="${2:-${DFV_PERF_N:-10}}"
target_init "$sys"
out="${DFV_PERF_OUT:-local/p8/perf}"
mkdir -p "$out"
base="$out/start-$T_LABEL"
trap 'target_stop >/dev/null 2>&1 || true; target_ax_restore' EXIT

target_build "$base.build.log"
target_ax_off
target_wake
setup=M03
[[ $sys == tvos ]] && setup=TV04
echo "perf-start: $T_LABEL: signing in ($setup)"
target_launch "$base-setup.log" "DFV_STATE=$setup"
target_wait "$base-setup.log" "dfv: shot ready $setup " 120 || { echo "the sign-in ($setup) did not end: $base-setup.log" >&2; exit 2; }
target_stop
sleep 2

csv="$base.csv"
echo "run,process_ms,command_ms,am_total_ms,from" >"$csv"
missing=0
for ((i = 1; i <= n; i++)); do
	log="$base-$i.log"
	sleep 1
	target_launch "$log"
	if ! target_wait "$log" "dancefitvn: mark home [0-9]+ t=" 60; then
		echo "perf-start: run $i: no mark in 60 s ($log)" >&2
		missing=1
		target_stop
		continue
	fi
	line=$(tr -d '\r' <"$log" | grep -oE "dancefitvn: mark home [0-9]+ t=[0-9]+ from=[a-z]+" | head -1)
	ms=$(awk '{ print $4 }' <<<"$line")
	t=$(sed -E 's/.* t=([0-9]+).*/\1/' <<<"$line")
	from=${line##*from=}
	cmd=""
	[[ $T_KIND == sim || ($T_KIND == android && $T_PHONE == 0) ]] && cmd=$((t - T_LAUNCH_MS)) # the Mac's clock
	am=""
	[[ -f $log.am ]] && am=$(tr -d '\r' <"$log.am" | sed -nE 's/^TotalTime: ([0-9]+)/\1/p' | head -1)
	echo "$i,$ms,$cmd,$am,$from" >>"$csv"
	echo "perf-start: run $i: home at ${ms} ms from the $from start${cmd:+ (command → mark ${cmd} ms)}${am:+, am start TotalTime ${am} ms}"
	target_stop
done

gate=1500
[[ $sys == android ]] && gate=2500
python3 -I - "$csv" "$base.md" "$T_LABEL" "$gate" "$n" "$T_AX" <<'PY'
import csv, statistics, sys
path, md, label, gate, n, ax = sys.argv[1], sys.argv[2], sys.argv[3], int(sys.argv[4]), int(sys.argv[5]), sys.argv[6]
rows = list(csv.DictReader(open(path)))
def col(name):
    return [int(r[name]) for r in rows if r[name]]
def p90(xs):
    xs = sorted(xs)
    return xs[min(len(xs) - 1, (len(xs) * 90 + 99) // 100 - 1)] if xs else None
def fmt(xs):
    return f"median {statistics.median(xs):.0f} ms, p90 {p90(xs)} ms, min {min(xs)}, max {max(xs)}" if xs else "-"
proc, cmd, am = col("process_ms"), col("command_ms"), col("am_total_ms")
ok = bool(proc) and statistics.median(proc) <= gate
lines = [f"# Launch to the first Home frame, {label}", "",
         f"{len(rows)} of {n} cold launches marked (signed in, the mock)."
         + (" The simulator's accessibility runtime off for the run (ApplicationAccessibilityEnabled, put back after)." if ax == "off" else ""), "",
         "| run | process start → Home (ms) | command → Home (ms) | am start TotalTime (ms) | counted from |",
         "|---|---|---|---|---|"]
lines += [f"| {r['run']} | {r['process_ms']} | {r['command_ms'] or '-'} | {r['am_total_ms'] or '-'} | {r['from']} |" for r in rows]
lines += ["", f"- process start → Home: {fmt(proc)}",
          f"- command → Home (the Mac's clock): {fmt(cmd)}",
          f"- am start TotalTime (Android, to the activity's first frame): {fmt(am)}",
          "", f"Gate (07 §5, the median of process start → Home): ≤ {gate} ms: {'PASS' if ok else 'FAIL'}"
          + (" (a simulator or an emulator: indicative, R-P8-5)" if "sim" in label or "emulator" in label else "")]
open(md, "w").write("\n".join(lines) + "\n")
print("\n".join(lines))
sys.exit(0 if ok else 1)
PY
status=$?
((missing)) && exit 2
exit $status
