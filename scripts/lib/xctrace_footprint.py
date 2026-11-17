# Adapted from scripts/tvos-memgate.sh (its exporter of the device's
# samples, itself from jplayer-tv/scripts/tvos-soak.sh at 781bee5).
#
# xctrace_footprint.py <toc.xml> <live.xml> <process name>: prints
# "t_ms,footprint_b" lines, one per sample of an Activity Monitor trace
# (xcrun xctrace export of the activity-monitor-process-live table), for
# the process whose name begins with <process name>: t_ms the Unix time in
# milliseconds (the trace's start plus the sample's offset), footprint_b
# its memory-physical-footprint in bytes.
import datetime
import re
import sys
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
