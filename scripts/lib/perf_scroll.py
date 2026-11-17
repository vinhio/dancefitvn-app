# Adapted from scripts/perf-scroll.sh (its analysis, itself from
# scripts/tvos-framestats.sh and jplayer-tv's framelog at 781bee5).
#
# perf_scroll.py <out.md> <label> <ax> <list>=<console log>...: the frames
# of a flung list (docs/plan/07-testing.md §5), from the frame log lines
# (internal/app/framelog.go) between "dfv: perf scroll start" and "end":
#
#   fs <unix ms> <frame n> <layout µs> <paint µs> <present µs> <total µs> <gpu> <unchanged> <pixels>
#
# Every frame the view drew counts for the pacing: the glides (a pause
# over 100 ms ends one) and the gaps inside a glide, two consecutive drawn
# frames more than 32 ms apart (a vsync lost) or a jump of the frame count
# (FrameStats.Frames: a frame drawn but not logged). An unchanged frame
# (gDesk places scroll content on whole device pixels, so a fling's crawl
# below a pixel a frame draws the same pixels) is a drawn frame on time,
# not a gap. The work (layout + paint + present) and the glide rate are of
# the changed frames. On Android the first frame after more than 200 ms
# without one is reported apart and kept out of the work gate: the
# synthetic flings inject no touch, so the system gives no input boost and
# that frame runs on cores at their floor clock (local/p8/perf/report.md
# note 10). Gate: no gap, no work over 32 ms, glide rate p50 >= 58 fps.
# Writes the table into out.md and prints it; exit 1 when the gate fails.
import re
import statistics
import sys

GLIDE_PAUSE_MS = 100  # a pause longer than this ends a glide
GAP_MS = 32  # two drawn frames further apart than this lost a vsync
COLD_IDLE_MS = 200  # Android: a frame after this long idle is a cold one
MIN_RATE = 58


def pct(xs, p):
    xs = sorted(xs)
    return xs[min(len(xs) - 1, (len(xs) * p) // 100)] if xs else 0


def load(path):
    """Returns t0, t1, flings, the frames and the dropped entries of a log."""
    t0 = t1 = None
    flings, dropped, frames = 0, 0, []
    for raw in open(path, errors="replace"):
        line = raw.strip().replace("\r", "")
        m = re.search(r"dfv: perf scroll start \S+ t=(\d+) flings=(\d+)", line)
        if m:
            t0, flings = int(m.group(1)), int(m.group(2))
        m = re.search(r"dfv: perf scroll end \S+ t=(\d+)", line)
        if m:
            t1 = int(m.group(1))
        m = re.match(r"# dropped (\d+) entries", line)
        if m:
            dropped += int(m.group(1))
        f = line.split()
        if len(f) == 10 and f[0] == "fs":
            # (t ms, frame n, work ms, unchanged)
            frames.append((int(f[1]), int(f[2]), int(f[6]) / 1000, f[8] == "true"))
    frames.sort(key=lambda fr: (fr[1], fr[0]))
    return t0, t1, flings, frames, dropped


def analyse(path, android):
    t0, t1, flings, frames, dropped = load(path)
    if t0 is None or t1 is None:
        return None
    drawn = [fr for fr in frames if t0 <= fr[0] <= t1]
    before = [fr for fr in frames if fr[0] < t0]
    prev_t = before[-1][0] if before else None
    glides, cur = [], []
    for fr in drawn:
        if cur and fr[0] - cur[-1][0] > GLIDE_PAUSE_MS:
            glides.append(cur)
            cur = []
        cur.append(fr)
    if cur:
        glides.append(cur)
    gaps, jumps, gap_ms = 0, 0, []
    for g in glides:
        for a, b in zip(g, g[1:]):
            if b[0] - a[0] > GAP_MS:
                gaps += 1
                gap_ms.append(b[0] - a[0])
            if b[1] - a[1] > 1:
                jumps += 1
    # The old count: gaps between changed frames only (the crawl's
    # unchanged frames dropped first), for comparison.
    old_gaps = 0
    for g in glides:
        ch = [fr for fr in g if not fr[3]]
        old_gaps += sum(1 for a, b in zip(ch, ch[1:]) if b[0] - a[0] > GAP_MS)
    rates = []
    for g in glides:
        ch = [fr for fr in g if not fr[3]]
        if len(ch) >= 5 and ch[-1][0] > ch[0][0]:
            rates.append((len(ch) - 1) * 1000 / (ch[-1][0] - ch[0][0]))
    cold, work = [], []
    for fr in drawn:
        is_cold = android and prev_t is not None and fr[0] - prev_t > COLD_IDLE_MS
        prev_t = fr[0]
        if fr[3]:
            continue
        (cold if is_cold else work).append(fr[2])
    over16 = sum(1 for w in work if w > 16.7)
    over32 = sum(1 for w in work if w > 32)
    rate_ok = bool(rates) and statistics.median(rates) >= MIN_RATE
    ok = bool(work) and gaps == 0 and jumps == 0 and over32 == 0 and rate_ok
    return dict(
        flings=flings, drawn=len(drawn), changed=sum(1 for fr in drawn if not fr[3]),
        unchanged=sum(1 for fr in drawn if fr[3]), glides=len(glides), rates=rates,
        gaps=gaps, gap_ms=gap_ms, jumps=jumps, old_gaps=old_gaps, dropped=dropped,
        work=work, over16=over16, over32=over32, cold=cold, ok=ok)


def main():
    md, label, ax, pairs = sys.argv[1], sys.argv[2], sys.argv[3], sys.argv[4:]
    android = "android" in label
    lines = [f"# Frames of a flung list, {label}", ""]
    if ax == "off":
        lines += ["The simulator's accessibility runtime off for the run (put back after).", ""]
    head = ("| list | flings | drawn frames (changed / unchanged) | glides | glide rate p50 / min (fps) "
            "| gaps > 32 ms | frame-count jumps | work > 16.7 ms | work > 32 ms | work p50 / p95 / max (ms) ")
    sep = "|---|---|---|---|---|---|---|---|---|---|"
    if android:
        head += "| first frame after > 200 ms idle: n, p50 / max (ms), > 32 ms "
        sep += "---|"
    lines += [head + "| gate |", sep + "---|"]
    notes, ok_all = [], True
    for pair in pairs:
        name, path = pair.split("=", 1)
        r = analyse(path, android)
        if r is None:
            cells = 10 + (1 if android else 0)
            lines.append(f"| {name} |" + " - |" * (cells - 1) + f" FAIL: no start or end in {path} |")
            ok_all = False
            continue
        ok_all = ok_all and r["ok"]
        rates, work, cold = r["rates"], r["work"], r["cold"]
        rate = f"{statistics.median(rates):.1f} / {min(rates):.1f}" if rates else "-"
        row = (f"| {name} | {r['flings']} | {r['drawn']} ({r['changed']} / {r['unchanged']}) | {r['glides']} | {rate} "
               f"| {r['gaps']} | {r['jumps']} | {r['over16']} | {r['over32']} "
               f"| {pct(work, 50):.1f} / {pct(work, 95):.1f} / {max(work, default=0):.1f} ")
        if android:
            row += (f"| {len(cold)}, {pct(cold, 50):.1f} / {max(cold, default=0):.1f}, "
                    f"{sum(1 for w in cold if w > 32)} ")
        lines.append(row + f"| {'PASS' if r['ok'] else 'FAIL'} |")
        gaps = ", ".join(str(g) for g in sorted(r["gap_ms"])) or "none"
        notes.append(f"- {name}: gaps over 32 ms between drawn frames: {gaps}; between changed frames only "
                     f"(the old count, a fling's crawl taken for a gap): {r['old_gaps']}"
                     + (f"; {r['dropped']} log entries dropped" if r["dropped"] else "") + ".")
    lines += ["", "Pacing (glides, gaps) is of every frame the view drew between the start and the end of the",
              "flings, an unchanged frame included (a fling's crawl below a device pixel a frame draws the",
              "same pixels: on time, nothing to present); a glide ends at a pause over 100 ms; a gap is two",
              "drawn frames more than 32 ms apart or a jump of the frame count. Work and glide rate are of",
              "the changed frames."]
    if android:
        lines += ["Android: the first frame after more than 200 ms without one is reported apart and is not",
                  "in the work columns or the gate. The synthetic flings inject no touch, so Android gives no",
                  "input boost and that frame runs on cores at their floor clock (a real touch is boosted)."]
    lines += ["Gate (07 §5): no gap, no work over 32 ms, glide rate p50 >= 58 fps."
              + (" A simulator or an emulator: indicative (R-P8-5)." if "sim" in label or "emulator" in label else ""),
              ""] + notes
    open(md, "w").write("\n".join(lines) + "\n")
    print("\n".join(lines))
    sys.exit(0 if ok_all else 1)


main()
