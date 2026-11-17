# shotcmp

Compares the `layout.json` and PNG of an app state with the prototype's
(docs/plan/07-testing.md section 3.3). Standard library only. Adapted from
`jplayer-mobile/cmd/shotcmp` (picture helpers, match by name and index).

    go run ./cmd/shotcmp -proto local/shots/prototype-kit/mobile \
        -app local/shots/kit/ios -states K-M02,K-M03 -mode layout,crop \
        -report local/shots/kit-report.md -json local/shots/kit-report.json

Both folders hold `<ID>.layout.json` and `<ID>.png`. Run `-h` for all flags.
Exit code: 0 pass, 1 differences, 2 usage or read error.

## Modes (`-mode`, comma separated)

- `layout`: elements match by `(name, index)`. x, y, w, h within 1 unit
  (mobile) or 2 units (tv); with text: fontFamily (first family), fontSize,
  fontWeight, color, letterSpacing, lineHeight, and the text (case and white
  space folded); always: background, radius, border, opacity. A style field
  the app leaves empty is not compared. Prototype elements the app does not
  have are `missing` (reported, a failure with `-require-all`); app elements
  the prototype lacks are `extra` (reported).
- `crop`: for every matched element, the prototype box and the app box are cut
  out, the app's is area-resampled to the prototype's pixel size, and the
  element fails when more than 1.5 % of the unmasked pixels differ by more
  than 32/255 in a channel. One diff picture per state
  (`<diff>/<ID>.png`: prototype | app | difference; bad pixels white, masked
  pixels blue, failing boxes outlined red).
- `full`: the same on the whole screen.

## Picture origin (`-proto-origin`, `-app-origin`)

The prototype's mobile shots are 394 px wide for a 393 wide `#m`: column 0 is
a 1 px device-frame column and layout x 0 is at column 1. Both pictures are
cut to the screen before any comparison, so layout (0, 0) is pixel (0, 0) in
crops, masks, the `full` mode and the diff pictures (which therefore are the
screen without the frame column). `auto` (default) takes the origin as PNG
size minus `round(screen * scale)` per axis when that is 1 or 2 px (up to the scale at 3x) (so
394 x 852 gives 1,0; 393, 1182 (3x: origin 3,0 is explicit) and TV 1920 x 1080 give 0,0; a bigger
difference is a different scale, not a frame). `x,y` sets it in pixels.

## Android (stretch rule)

When the app screen is wider than the prototype's (412 against 393), boxes
are compared after the 07 section 3.3 rule (`-margin`, default 24): an
element anchored to both screen edges grows by the width difference, one
anchored to the right edge only moves by it, a centred one moves by half,
all others keep their box. Pixel modes are skipped (note in the report).
`-no-stretch` turns the rule off.

## Masks (pixel modes, prototype units)

`-status-bar H` (default 54 mobile, 0 tv), `-corner R` (49 mobile, 0 tv; the
device frame), `-mask [ID:]x,y,w,h` (repeatable), `-mask-name REGEXP` (boxes of
prototype elements, e.g. the video area, QR, clock). `-include` / `-exclude`
filter elements by name.

## layout.json schema (shared with `internal/ui/kit/layoutdump`)

Go types: `Layout`, `Screen`, `Element`, `Style` in `schema.go`.

    { "id": "K-M02", "platform": "mobile|tv", "unit": "css px",
      "screen": {"w": 393, "h": 852},
      "elements": [ {
        "key": "go.account", "name": "go.account", "index": 1,
        "path": "", "parent": "", "tag": "", "text": "visible text",
        "x": 0, "y": 0, "w": 0, "h": 0,
        "style": { "fontFamily": "Barlow Condensed", "fontSize": "72px",
                   "fontWeight": "800", "lineHeight": "72px",
                   "letterSpacing": "2px", "color": "rgb(255, 208, 40)",
                   "background": "rgba(0, 0, 0, 0)", "radius": "12px",
                   "border": "1px rgb(255, 255, 255)", "opacity": "1" } } ] }

- `name` is the widget key; `index` the 1-based occurrence of the name in the
  file; `key` is `name` (`name#index` from the second occurrence). `x`, `y`
  are relative to the screen root, in points on the app (`screen` = window
  size in points, 412x892 on Android) and CSS px on the prototype.
- Style values are CSS strings: sizes `"14px"` (`"normal"` allowed for
  spacing and line height), colors `rgb(r, g, b)` / `rgba(r, g, b, a)` /
  `#rrggbb[aa]`, weight `"700"`. Colors match within 1 per channel and 0.01
  alpha. Empty or omitted means not recorded. Extra fields are ignored.
- The PNG scale is the PNG width over `screen.w` (`-proto-scale`,
  `-app-scale` override), so 1x, 2x and 3x pictures mix freely.

## 3x pixel gate, text threshold, waivers

    shotcmp -proto local/shots/prototype-kit/mobile -proto-3x local/shots/prototype-kit/mobile@3x \
        -app local/shots/app-kit/ios -mode layout,crop -except-file local/shots/kit-exceptions-ios.txt

- `-proto-3x DIR`: the prototype's PNGs come from DIR (1182 px wide: 3 px frame column; scale 3
  and origin 3,0 are implied), the layout from `-proto`. The app picture is its 3x one, so no
  resampling blurs thin strokes.
- `-text-max 0.08`: share of differing pixels allowed in the crop of an element that shows text
  (Core Text and Chrome rasterise glyphs differently); other crops keep 1.5 %.
- `-except ID:key` (repeatable) and `-except-file`: failures of these elements are reported as
  `waived` and do not fail the state. Each line needs a documented cause (PROGRESS section 5).
- The Android stretch rule (`stretch.go`) is mirrored by `internal/ui/mobile/kitdemo/stretch.go`;
  `TestPlanXMatchesShotcmp` runs the same case through both.
