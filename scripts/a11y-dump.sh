#!/usr/bin/env bash
# Adapted from jplayer-mobile/scripts/m11-a11y.sh at 49de3c2: the states
# are the shot states of a developer's build (DFV_STATE) instead of a
# script's steps, the dump is written beside the state's layout
# (DFV_A11Y=1, internal/app/devaccess.go), the TV's focus is moved with
# key presses after it (DFV_A11Y_MOVE), and the dumps are checked.
#
# a11y-dump.sh dumps DanceFITVN's accessibility on the iPhone or the Apple
# TV simulator, where no VoiceOver runs (local/plans/P8.md P8.3d): for each
# state, the content's tree (ui.View.Semantics) and the elements the uikit
# bridge hands VoiceOver (internal/sys.AccessibilityDump: label, value,
# traits, frame), as JSON; then checks them:
#   1. every element the system gets has a label and a trait;
#   2. nothing under a sheet, the panel or the hidden controls is read: no
#      element of M07 (the options sheet), M06-hidden (controls hidden) or
#      TV08 (the panel) has the label and the frame of one of M06 or TV07
#      (but M06-hidden's stage, the whole screen, there in both);
#   3. on the TV, the element of the focused node carries the selected
#      trait (the bridge's focus, which VoiceOver follows), before and after
#      each key press, and the focus moves.
#
#   scripts/a11y-dump.sh ios|tvos [ID ...]
#
# The states by default: ios M03 M05 M06 M06-hidden M07 M15; tvos TV04 TV06
# TV07 TV08. On the TV each is dumped again after each key of its moves
# (TV04 right,down; TV06 down,down; TV07 right,left; TV08 down,up;
# DFV_A11Y_MOVE replaces them all), as <ID>-m1, <ID>-m2.
#
# Simulators only (no VoiceOver dump of a device: it would need its
# accessibility settings changed): "DanceFITVN iPhone 16" (DFV_SIM) or
# "DanceFITVN Apple TV" (DFV_TVSIM), booted. UIKit computes an element's
# screen frame only with its accessibility runtime loaded, as VoiceOver or
# Accessibility Inspector load it: the simulator's own defaults
# com.apple.Accessibility ApplicationAccessibilityEnabled is turned on for
# the run and put back after (deleted when it was unset).
#
# DFV_NOBUILD=1 installs dist/ as it is. Writes into DFV_A11Y_OUT (default
# local/p8/a11y)/<ios|tvos>/: <ID>.a11y.json, <ID>.png, <ID>.log (the
# console), report.md (the checks). Exit 1 when a check fails.
set -uo pipefail
cd "$(dirname "$0")/.."
# shellcheck source=lib/target.sh
source scripts/lib/target.sh

sys="${1:?usage: scripts/a11y-dump.sh ios|tvos [ID ...]}"
shift
case $sys in
ios) ids=("$@"); [[ ${#ids[@]} -gt 0 ]] || ids=(M03 M05 M06 M06-hidden M07 M15) ;;
tvos) ids=("$@"); [[ ${#ids[@]} -gt 0 ]] || ids=(TV04 TV06 TV07 TV08) ;;
*) echo "usage: scripts/a11y-dump.sh ios|tvos [ID ...]" >&2; exit 2 ;;
esac
# Simulators only: the device's variables are not read here.
unset DFV_IPHONE_UDID DFV_TV_UDID
target_init "$sys"
out="${DFV_A11Y_OUT:-local/p8/a11y}/$sys"
mkdir -p "$out"

# moves_of prints the keys pressed after the dump of state $1.
moves_of() {
	if [[ -n ${DFV_A11Y_MOVE:-} ]]; then echo "$DFV_A11Y_MOVE"; return; fi
	case $1 in
	TV04) echo right,down ;;
	TV06) echo down,down ;;
	TV07) echo right,left ;;
	TV08) echo down,up ;;
	*) echo "" ;;
	esac
}

target_build "$out/build.log"
data=""
for _ in 1 2 3 4 5; do data=$(xcrun simctl get_app_container "$T_ID" "$T_BUNDLE" data 2>/dev/null) && break; sleep 2; done
[[ -n $data ]] || { echo "DanceFITVN is not installed on $T_ID" >&2; exit 2; }
dir="$data/Library/Caches/$T_BUNDLE/dfv-shots"

# The simulator's accessibility runtime, on for the run, put back after.
was=$(xcrun simctl spawn "$T_ID" defaults read com.apple.Accessibility ApplicationAccessibilityEnabled 2>/dev/null)
restore() {
	target_stop >/dev/null 2>&1 || true
	if [[ -z $was ]]; then
		xcrun simctl spawn "$T_ID" defaults delete com.apple.Accessibility ApplicationAccessibilityEnabled 2>/dev/null
	else
		case $was in 1 | true | YES) was=true ;; *) was=false ;; esac
		xcrun simctl spawn "$T_ID" defaults write com.apple.Accessibility ApplicationAccessibilityEnabled -bool "$was"
	fi
}
trap restore EXIT
xcrun simctl spawn "$T_ID" defaults write com.apple.Accessibility ApplicationAccessibilityEnabled -bool true

fail=0
for id in "${ids[@]}"; do
	rm -f "$dir/$id.a11y.json" "$dir/$id"-m*.a11y.json "$out/$id.a11y.json" "$out/$id"-m*.a11y.json
	moves=$(moves_of "$id")
	vars=("DFV_STATE=$id" "DFV_A11Y=1")
	[[ -n $moves ]] && vars+=("DFV_A11Y_MOVE=$moves")
	target_launch "$out/$id.log" "${vars[@]}"
	if ! target_wait "$out/$id.log" "dfv: shot ready $id " 90; then
		echo "a11y: $id: the state was not reached in 90 s (see $out/$id.log)" >&2
		fail=1
		continue
	fi
	target_shot "$out/$id.png"
	target_wait "$out/$id.log" "dfv: a11y done $id\$" 30 || { echo "a11y: $id: the dumps did not end (see $out/$id.log)" >&2; fail=1; }
	target_stop || true
	for f in "$dir/$id.a11y.json" "$dir/$id"-m*.a11y.json; do
		[[ -f $f ]] && cp "$f" "$out/"
	done
	echo "a11y: $sys $id -> $(cd "$out" && ls "$id".a11y.json "$id"-m*.a11y.json 2>/dev/null | tr '\n' ' ')"
done

python3 -I - "$out" "$sys" "${ids[@]}" <<'PY' || fail=1
import json, os, sys
out, system, ids = sys.argv[1], sys.argv[2], sys.argv[3:]
# The states read over another: their elements must not repeat its.
BASE = {"M07": "M06", "M06-hidden": "M06", "TV08": "TV07"}
def load(name):
    p = os.path.join(out, name + ".a11y.json")
    return json.load(open(p)) if os.path.exists(p) else None
def key(e):
    r = e.get("inView") or {}
    return (e.get("label", ""), round(r.get("X", 0)), round(r.get("Y", 0)), round(r.get("W", 0)), round(r.get("H", 0)))
def focused(rep):
    for n in rep.get("tree", []):
        if n.get("focused"):
            return n
    return None
lines, bad = [f"# Accessibility dump, {system} simulator", ""], 0
for sid in ids:
    names = [sid] + sorted((f[:-len(".a11y.json")] for f in os.listdir(out)
                            if f.startswith(sid + "-m") and f.endswith(".a11y.json")),
                           key=lambda s: int(s.rsplit("-m", 1)[1]))
    prev_focus = None
    for name in names:
        rep = load(name)
        if rep is None:
            lines += [f"## {name}", "", "FAIL: no dump", ""]
            bad += 1
            continue
        els, tree = rep.get("system") or [], rep.get("tree") or []
        issues = []
        if not els:
            issues.append(f"no system element ({rep.get('systemNote', '')})")
        for i, e in enumerate(els):
            what = f"#{i} {e.get('label', '')!r} value={e.get('value', '')!r} traits={e.get('traits') or []}"
            if not e.get("label", "").strip():
                issues.append("no label: " + what)
            if not e.get("traits"):
                issues.append("no trait: " + what)
        zero = sum(1 for e in els if not (e.get("frame") or {}).get("W"))
        if els and zero == len(els):
            issues.append("every screen frame is zero: the accessibility runtime was not loaded")
        base = BASE.get(sid) if name == sid else None
        if base and load(base):
            under = {key(e) for e in load(base).get("system") or []}
            size = rep.get("size") or {}
            for e in els:
                r = e.get("inView") or {}
                if sid == "M06-hidden" and r.get("W") == size.get("W") and r.get("H") == size.get("H"):
                    continue  # the stage: the one element while the controls hide (P8.3a)
                if key(e) in under:
                    issues.append(f"read under the {'controls' if sid == 'M06-hidden' else 'sheet/panel'} (as in {base}): {e.get('label', '')!r} traits={e.get('traits') or []}")
        if system == "tvos":
            f = focused(rep)
            if f is None:
                issues.append("no focused node in the tree")
            else:
                sel = [e for e in els if "selected" in (e.get("traits") or [])]
                if f.get("label") and not any(e.get("label") == f.get("label") for e in sel):
                    issues.append(f"focus {f.get('label')!r} ({f.get('role')}) has no element with the selected trait; selected: {[e.get('label') for e in sel]}")
                if not f.get("label"):
                    issues.append(f"focused node without a label ({f.get('role')} @{f.get('bounds')})")
                if prev_focus is not None and (f.get("label"), json.dumps(f.get("bounds"))) == prev_focus:
                    issues.append("the focus did not move after the key press")
                prev_focus = (f.get("label"), json.dumps(f.get("bounds")))
        status = "PASS" if not issues else "FAIL"
        bad += bool(issues)
        f = focused(rep)
        lines += [f"## {name}: {status}", "",
                  f"{len(tree)} nodes in the tree, {len(els)} system elements" +
                  (f"; focus {f.get('label')!r}" if system == "tvos" and f else "") + ".", ""]
        lines += [f"- FAIL {x}" for x in issues]
        lines += ["", "| # | label | value | traits |", "|---|---|---|---|"]
        lines += [f"| {i} | {e.get('label', '')} | {e.get('value', '')} | {' '.join(e.get('traits') or [])} |" for i, e in enumerate(els)]
        lines.append("")
lines.insert(1, f"\n{'PASS' if not bad else 'FAIL'}: {bad} dump(s) with issues.")
open(os.path.join(out, "report.md"), "w").write("\n".join(lines) + "\n")
print("\n".join(l for l in lines if l.startswith("## ") or l.startswith("- FAIL") or l.startswith("\n")))
print("report:", os.path.join(out, "report.md"))
sys.exit(1 if bad else 0)
PY
exit $fail
