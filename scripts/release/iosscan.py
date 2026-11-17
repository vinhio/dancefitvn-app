# Adapted from jplayer-mobile/scripts/release/iosscan.py at 49de3c2.
"""Checks of an iOS or tvOS release build that need parsing, for check-ios.sh.

    python3 -I iosscan.py assets <assetutil.json> <icon name> <color name>
    python3 -I iosscan.py tvassets <assetutil.json> <icon> <color> <top shelf> <top shelf wide>
    python3 -I iosscan.py privacy <manifest.json> <imports.txt> <program>
    python3 -I iosscan.py selectors <sdk path> <go files list>

Each prints lines "ok    ...", "WARN  ..." or "FAIL  ..." and exits 0;
check-ios.sh counts the FAIL lines. Standard library only.
"""

import json
import os
import re
import sys


def out(kind, msg):
    print({"ok": "ok   ", "warn": "WARN ", "fail": "FAIL "}[kind], msg)


# --------------------------------------------------------------------
# Assets.car: the app icon (a 1024 marketing image) and the launch
# screen's color, as assetutil --info lists them.
# --------------------------------------------------------------------


def assets(path, icon, color):
    with open(path, encoding="utf-8") as f:
        entries = [e for e in json.load(f) if isinstance(e, dict)]
    sizes = sorted({(e.get("PixelWidth"), e.get("PixelHeight")) for e in entries
                    if e.get("Name") == icon and e.get("PixelWidth")})
    if not sizes:
        out("fail", f"Assets.car has no image of the icon {icon!r} (ITMS-90713)")
    elif (1024, 1024) not in sizes:
        out("fail", f"Assets.car icon {icon!r} has no 1024x1024 image (ITMS-90704): {sizes}")
    else:
        out("ok", f"Assets.car icon {icon!r}: {len(sizes)} sizes, 1024x1024 present")
    color_line(entries, color)


def color_line(entries, color):
    colors = [e for e in entries if e.get("AssetType") == "Color" and e.get("Name") == color]
    if not color:
        out("fail", "UILaunchScreen has no UIColorName")
    elif not colors:
        out("fail", f"Assets.car has no color {color!r} (UILaunchScreen.UIColorName)")
    else:
        comps = colors[0].get("Color components") or []
        hexs = "".join(f"{round(c * 255):02X}" for c in comps[:3]) if comps else "?"
        out("ok", f"Assets.car color {color!r}: #{hexs} ({colors[0].get('Colorspace', '?')})")


# --------------------------------------------------------------------
# tvOS brand assets: the App Icon stack (400x240 points on the TV, a
# 1280x768 App Store image) and the two Top Shelf images, by the names
# CFBundleIcons and TVTopShelfImage give (kits/cli README, tvOS row).
# --------------------------------------------------------------------


def tvassets(path, icon, color, shelf, wide):
    with open(path, encoding="utf-8") as f:
        entries = [e for e in json.load(f) if isinstance(e, dict)]

    def sizes(name, idiom):
        return sorted({(e.get("PixelWidth"), e.get("PixelHeight")) for e in entries
                       if e.get("Name") == name and e.get("Idiom") == idiom and e.get("PixelWidth")})

    for name, idiom, want in ((icon, "tv", (400, 240)), (icon, "marketing", (1280, 768)),
                              (shelf, "tv", (1920, 720)), (wide, "tv", (2320, 720))):
        have = sizes(name, idiom)
        if not name:
            out("fail", f"Info.plist names no image for {want[0]}x{want[1]} ({idiom})")
        elif want in have:
            out("ok", f"Assets.car {name!r} ({idiom}): {want[0]}x{want[1]} present")
        else:
            out("fail", f"Assets.car {name!r} ({idiom}) has no {want[0]}x{want[1]} image: {have}")
    if any(e.get("AssetType") == "ImageStack" and e.get("Name") == icon for e in entries):
        out("ok", f"Assets.car {icon!r} is an image stack")
    else:
        out("fail", f"Assets.car {icon!r} is no image stack (tvOS app icons are layered)")
    color_line(entries, color)


# --------------------------------------------------------------------
# Required reason APIs (developer.apple.com, "Describing use of required
# reason API"): each category the binary reaches must be declared in
# PrivacyInfo.xcprivacy with a reason (ITMS-91053). C functions show as
# imports; the Objective-C names, which purego looks up at run time,
# show as bytes of the program. Go lays its string constants end to end,
# without separators, so each name is searched as it is rather than cut
# out of the program by a pattern (NSURLVolumeTotalCapacityKey sits right
# before the next constant).
# --------------------------------------------------------------------

CATEGORY = "NSPrivacyAccessedAPICategory"
C_APIS = {
    "FileTimestamp": ["stat", "fstat", "fstatat", "lstat", "getattrlist", "fgetattrlist",
                      "getattrlistat", "getattrlistbulk"],
    "SystemBootTime": ["mach_absolute_time"],
    "DiskSpace": ["statfs", "statvfs", "fstatfs", "fstatvfs"],
}
OBJC_APIS = {
    "FileTimestamp": ["NSFileCreationDate", "NSFileModificationDate", "NSURLContentModificationDateKey",
                      "NSURLCreationDateKey", "fileModificationDate"],
    "SystemBootTime": ["systemUptime"],
    "DiskSpace": ["NSURLVolumeAvailableCapacityKey", "NSURLVolumeAvailableCapacityForImportantUsageKey",
                  "NSURLVolumeAvailableCapacityForOpportunisticUsageKey", "NSURLVolumeTotalCapacityKey",
                  "NSFileSystemFreeSize", "NSFileSystemSize"],
    "ActiveKeyboards": ["activeInputModes"],
    "UserDefaults": ["NSUserDefaults"],
}
# getattrlist and its kin are FileTimestamp and DiskSpace APIs both.
BOTH = {"getattrlist", "fgetattrlist", "getattrlistat"}
# Reasons each category may give (the codes this app can honestly claim
# are a subset; any other code is a typo). From Apple's
# NSPrivacyAccessedAPITypeReasons page, as the API lists above are from
# its NSPrivacyAccessedAPIType page.
REASONS = {
    "FileTimestamp": {"DDA9.1", "C617.1", "3B52.1", "0A2A.1"},
    "SystemBootTime": {"35F9.1", "8FFB.1", "3D61.1"},
    "DiskSpace": {"85F4.1", "E174.1", "7D9E.1", "B728.1"},
    "ActiveKeyboards": {"3EC4.1", "54BD.1"},
    "UserDefaults": {"CA92.1", "1C8F.1", "C56D.1", "AC6B.1"},
}


# The data types and purposes of Apple's NSPrivacyCollectedDataType and
# NSPrivacyCollectedDataTypePurpose pages ("Describing data use in privacy
# manifests"). DanceFITVN collects personal data (an account, a profile,
# lesson progress: local/plans/P8.md F-9), so the list cannot be empty.
DATA = "NSPrivacyCollectedDataType"
DATA_TYPES = {
    "Name", "EmailAddress", "PhoneNumber", "PhysicalAddress", "OtherUserContactInfo", "Health", "Fitness",
    "PaymentInfo", "CreditInfo", "OtherFinancialInfo", "PreciseLocation", "CoarseLocation", "SensitiveInfo",
    "Contacts", "EmailsOrTextMessages", "PhotosorVideos", "AudioData", "GameplayContent", "CustomerSupport",
    "OtherUserContent", "BrowsingHistory", "SearchHistory", "UserID", "DeviceID", "PurchaseHistory",
    "ProductInteraction", "AdvertisingData", "OtherUsageData", "CrashData", "PerformanceData",
    "OtherDiagnosticData", "EnvironmentScanning", "Hands", "Head", "OtherDataTypes",
}
PURPOSE = "NSPrivacyCollectedDataTypePurpose"
PURPOSES = {"ThirdPartyAdvertising", "DeveloperAdvertising", "Analytics", "ProductPersonalization",
            "AppFunctionality", "Other"}


def collected(entries):
    if not isinstance(entries, list) or not entries:
        out("fail", "PrivacyInfo: NSPrivacyCollectedDataTypes is empty or missing (the app collects account data)")
        return
    seen = []
    for e in entries:
        kind = str(e.get(DATA, "")).removeprefix(DATA)
        purposes = [str(p).removeprefix(PURPOSE) for p in e.get(DATA + "Purposes", [])]
        bad = [p for p in purposes if p not in PURPOSES]
        if kind not in DATA_TYPES:
            out("fail", f"PrivacyInfo: unknown collected data type {e.get(DATA)!r}")
        elif kind in seen:
            out("fail", f"PrivacyInfo: collected data type {kind} listed twice")
        elif not isinstance(e.get(DATA + "Linked"), bool) or not isinstance(e.get(DATA + "Tracking"), bool):
            out("fail", f"PrivacyInfo: {kind}: Linked and Tracking must be booleans")
        elif e.get(DATA + "Tracking") is not False:
            out("fail", f"PrivacyInfo: {kind}: Tracking is true (the app does not track)")
        elif not purposes or bad:
            out("fail", f"PrivacyInfo: {kind}: purposes {purposes} (unknown: {bad})")
        seen.append(kind)
    out("ok", f"PrivacyInfo: {len(seen)} collected data types ({', '.join(seen)}), none for tracking")


def privacy(manifest_path, imports_path, program_path):
    with open(manifest_path, encoding="utf-8") as f:
        m = json.load(f)
    if m.get("NSPrivacyTracking") is not False:
        out("fail", "PrivacyInfo: NSPrivacyTracking is not false")
    if m.get("NSPrivacyTrackingDomains"):
        out("fail", "PrivacyInfo: NSPrivacyTrackingDomains is not empty")
    collected(m.get("NSPrivacyCollectedDataTypes"))
    declared = {}
    for d in m.get("NSPrivacyAccessedAPITypes", []):
        cat = d.get("NSPrivacyAccessedAPIType", "").removeprefix(CATEGORY)
        reasons = d.get("NSPrivacyAccessedAPITypeReasons", [])
        declared[cat] = reasons
        bad = [r for r in reasons if r not in REASONS.get(cat, set())]
        if cat not in REASONS:
            out("fail", f"PrivacyInfo: unknown category {CATEGORY}{cat}")
        elif not reasons or bad:
            out("fail", f"PrivacyInfo: {cat} reasons {reasons} (unknown: {bad})")

    with open(imports_path, encoding="utf-8") as f:
        imports = {s.lstrip("_").split("$")[0] for s in f.read().split()}
    with open(program_path, "rb") as f:
        data = f.read()
    names = {a for apis in OBJC_APIS.values() for a in apis if a.encode() in data}
    used = {}
    for cat, apis in C_APIS.items():
        hit = sorted(a for a in apis if a in imports)
        if hit:
            used.setdefault(cat, []).extend(hit)
    for cat, apis in OBJC_APIS.items():
        hit = sorted(a for a in apis if a in names)
        if hit:
            used.setdefault(cat, []).extend(hit)
    both = sorted(BOTH & imports)
    if both and "DiskSpace" not in declared:
        out("warn", f"required reason: {', '.join(both)} also count as DiskSpace when they read volume sizes")
    for cat in sorted(set(used) | set(declared)):
        if cat in used and cat in declared:
            out("ok", f"required reason {cat}: used ({', '.join(used[cat])}), declared {declared[cat]}")
        elif cat in used:
            out("fail", f"required reason {cat}: the binary uses {', '.join(used[cat])} but PrivacyInfo does not declare it (ITMS-91053)")
        else:
            out("warn", f"required reason {cat}: declared {declared[cat]} but nothing in the binary uses it")


# --------------------------------------------------------------------
# Selectors: every selector the Go code sends or registers should be in
# the SDK's headers; one that is not is a typo or a private API (App
# Review 2.5.1). A heuristic: each "piece:" of a selector must appear in
# one header file; a selector without arguments as an identifier.
# --------------------------------------------------------------------

LIT_SEL = re.compile(r'\b(?:Sel|RegisterName|SelRegisterName)\("([A-Za-z_][A-Za-z0-9_:]*)"\)')
LIT_CALL = re.compile(r'\b(?:call|send|msg|msgSend)[A-Za-z0-9]*\([^,()"]+,\s*"([A-Za-z_][A-Za-z0-9_:]*)"')
LIT_COLON_FULL = re.compile(r'"([A-Za-z_][A-Za-z0-9_]*:(?:[A-Za-z_][A-Za-z0-9_]*:)*)"')
COMMENT = re.compile(r"^\s*//.*$", re.M)
DEF_LIT = re.compile(r'\bSelector:\s*"([^"]+)"')
DEF_IDENT = re.compile(r"\bSelector:\s*([A-Za-z_][A-Za-z0-9_]*)\b")
PIECE = re.compile(r"([A-Za-z_][A-Za-z0-9_]*)\s*:")
IDENT = re.compile(r"[A-Za-z_][A-Za-z0-9_]*")


def selectors(sdk, list_path):
    found, defined = {}, set()
    with open(list_path, encoding="utf-8") as f:
        files = [l.strip() for l in f if l.strip()]
    for path in files:
        with open(path, encoding="utf-8") as g:
            src = g.read()
        if "ffi/objc" not in src and "purego/objc" not in src:
            continue
        src = COMMENT.sub("", src)
        for rx in (LIT_COLON_FULL, LIT_SEL, LIT_CALL):
            for s in rx.findall(src):
                found.setdefault(s, set()).add(os.path.relpath(path))
        defined.update(DEF_LIT.findall(src))
        for ident in DEF_IDENT.findall(src):
            m = re.search(r"\b" + re.escape(ident) + r'\s*=\s*"([^"]+)"', src)
            if m:
                defined.add(m.group(1))
    if not found:
        out("warn", "selectors: none found in the Go sources")
        return

    pieces, idents = {}, set()
    # iOS 26 moved some of UIKit's protocols (UICoordinateSpace) into
    # SubFrameworks (UIUtilities).
    roots = [os.path.join(sdk, "System/Library/Frameworks"), os.path.join(sdk, "System/Library/SubFrameworks"),
             os.path.join(sdk, "usr/include/objc")]
    for root in roots:
        for d, _, names in os.walk(root):
            for n in names:
                if not n.endswith(".h"):
                    continue
                p = os.path.join(d, n)
                try:
                    with open(p, encoding="utf-8", errors="replace") as h:
                        text = h.read()
                except OSError:
                    continue
                for piece in set(PIECE.findall(text)):
                    pieces.setdefault(piece, set()).add(p)
                idents.update(IDENT.findall(text))

    missing = []
    for sel in sorted(found):
        parts = sel.split(":")[:-1] if sel.endswith(":") else None
        if parts is None:
            ok = sel in idents
        else:
            sets = [pieces.get(p) for p in parts if p]
            ok = all(sets) and bool(set.intersection(*sets)) if sets else True
            if not ok and len(parts) == 1 and parts[0].startswith("set") and len(parts[0]) > 3:
                prop = parts[0][3].lower() + parts[0][4:]
                ok = prop in idents
        if not ok:
            missing.append(sel)
    own = [m for m in missing if m in defined]
    missing = [m for m in missing if m not in defined]
    out("ok", f"selectors: {len(found) - len(missing) - len(own)} of {len(found)} found in the "
              f"{os.path.basename(sdk)} headers, {len(own)} defined by the Go code ({', '.join(own)})")
    for sel in missing:
        where = ", ".join(sorted(found[sel])[:2])
        out("warn", f"selector {sel!r} not in the SDK headers (a method defined in Go, or private?): {where}")


def main(argv):
    cmd = argv[1] if len(argv) > 1 else ""
    if cmd == "assets" and len(argv) == 5:
        assets(*argv[2:])
    elif cmd == "tvassets" and len(argv) == 7:
        tvassets(*argv[2:])
    elif cmd == "privacy" and len(argv) == 5:
        privacy(*argv[2:])
    elif cmd == "selectors" and len(argv) == 4:
        selectors(*argv[2:])
    else:
        print(__doc__, file=sys.stderr)
        return 2
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
