#!/usr/bin/env bash
# Adapted from scripts/tvos-framestats.sh (the frame log's analysis, from
# jplayer-tv's framelog at 781bee5); the launch is scripts/lib/target.sh's.
#
# perf-scroll.sh times the frames of a list flung by a finger
# (docs/plan/07-testing.md §5: 60 fps, no frame over 32 ms; local/plans/
# P8.md P8.4a): a developer's build at DFV_STATE=PERF-SCROLL (Courses, M04)
# or PERF-SCROLL-M05 (the detail of Set 35) flings the list up and down
# DFV_PERF_FLINGS times (20), 1.5 s apart, with synthetic touches timed as
# a finger's (internal/app/devperf.go), its frame log on (DFV_FRAMELOG=1
# GDESK_FRAMESTATS=1: a line per frame with its layout, paint and present
# times). scripts/lib/perf_scroll.py reads the frames between "dfv: perf
# scroll start" and "end": the pacing from every frame drawn, unchanged
# ones included (a fling's crawl below a device pixel a frame presents
# nothing, on time), cut into glides (a pause over 100 ms ends one), a gap
# being two drawn frames more than 32 ms apart or a jump of the frame
# count; the work (layout + paint + present on the UI thread) and the
# glide rate from the changed frames. On Android the first frame after a
# pause over 200 ms is reported apart, out of the work gate (no touch is
# injected, so no input boost: a cold CPU). The gate: no gap and no work
# over 32 ms, and a glide rate of 58 fps at least.
#
#   scripts/perf-scroll.sh ios|android [courses|detail|both]
#   scripts/perf-scroll.sh analyse <label> <list>=<console log>...
#
# The second form only analyses logs already taken (label as the run's:
# iphone, ios-sim, android-phone, android-emulator) into
# DFV_PERF_OUT/scroll-<label>.md.
#
# The target (scripts/lib/target.sh): the iPhone simulator or
# DFV_IPHONE_UDID's iPhone; ANDROID_SERIAL's emulator or phone. On a
# simulator its accessibility runtime is off for the run and put back
# after. DFV_NOBUILD=1 installs dist/ as it is. Writes into DFV_PERF_OUT
# (default local/p8/perf): scroll-<target>-<list>.log (the console),
# scroll-<target>.md. Exit 1 when the gate fails.
set -uo pipefail
cd "$(dirname "$0")/.."
# shellcheck source=lib/target.sh
source scripts/lib/target.sh

sys="${1:?usage: scripts/perf-scroll.sh ios|android [courses|detail|both] | analyse <label> <list>=<log>...}"
if [[ $sys == analyse ]]; then
	label="${2:?usage: scripts/perf-scroll.sh analyse <label> <list>=<log>...}"
	shift 2
	(($# > 0)) || { echo "usage: scripts/perf-scroll.sh analyse <label> <list>=<log>..." >&2; exit 2; }
	out="${DFV_PERF_OUT:-local/p8/perf}"
	mkdir -p "$out"
	ax=device
	[[ $label == *sim* ]] && ax=off
	exec python3 -I scripts/lib/perf_scroll.py "$out/scroll-$label.md" "$label" "$ax" "$@"
fi
which="${2:-both}"
case $sys in ios | android) ;; *) echo "usage: scripts/perf-scroll.sh ios|android [courses|detail|both]" >&2; exit 2 ;; esac
case $which in
courses) lists=(courses) ;;
detail) lists=(detail) ;;
both) lists=(courses detail) ;;
*) echo "the list is courses, detail or both" >&2; exit 2 ;;
esac
flings=${DFV_PERF_FLINGS:-20}
target_init "$sys"
out="${DFV_PERF_OUT:-local/p8/perf}"
mkdir -p "$out"
base="$out/scroll-$T_LABEL"
trap 'target_stop >/dev/null 2>&1 || true; target_ax_restore' EXIT
target_build "$base.build.log"
target_ax_off
target_wake

logs=()
for list in "${lists[@]}"; do
	state=PERF-SCROLL
	[[ $list == detail ]] && state=PERF-SCROLL-M05
	log="$base-$list.log"
	target_launch "$log" "DFV_STATE=$state" "DFV_FRAMELOG=1" "GDESK_FRAMESTATS=1" "DFV_PERF_FLINGS=$flings"
	if ! target_wait "$log" "dfv: perf scroll end $state t=" $((flings * 2 + 90)); then
		echo "perf-scroll: $state did not end (see $log)" >&2
	fi
	sleep 1.5 # the frame log writes once a second
	target_shot "$base-$list.png"
	target_stop
	logs+=("$list=$log")
done

python3 -I scripts/lib/perf_scroll.py "$base.md" "$T_LABEL" "$T_AX" "${logs[@]}"
