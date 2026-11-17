# Prototype Night Stage (spec)

`index.html` is the approved DanceFITVN prototype (Mobile + Apple TV), a single vanilla HTML file. It is the specification: never edit it in the app repo.

- Run: `python3 -m http.server 5180` here, open http://localhost:5180. Top bar: Cả hai / Mobile / TV, "Ngoại tuyến" switch, "Làm lại". TV: arrow keys, Enter, Esc, Space, or the remote under the TV frame.
- `screenshots/{mobile,tv}/<ID>.png`: the 43 states of docs/plan/07-testing.md §3.2 at scale 1, captured with `reference/tools/capture_dancefitvn.py` (no page errors, frozen clock).
- Changes since approval: 2026-11-14, the user chose auto-hiding video controls (Q12): `.vui` elements fade out (250 ms) 3 s after the last touch while a video plays; tapping the video toggles them (`ctlHidden`, `stageTap`, `CTL_HIDE_MS`; new state M06-hidden). The capture run found no script errors.
